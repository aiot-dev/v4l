// Package v4l gives access to V4L (Video For Linux).
// It accessess v4l directly via open(2) and ioctl(2).
// It does not use cgo wrappings of the C v4l library.
package v4l

import (
	"fmt"
	"image"
	"os"
	"sync"
)

// A Format is one of the pixel formats specified by V4L.
// Only pixel formats supported by this package are here.
type Format uint32

const (
	V4L2_PIX_FMT_UYVY Format = 0x55595659 // 'UYVY'
)

var ssMap = map[Format]image.YCbCrSubsampleRatio{
	V4L2_PIX_FMT_UYVY: image.YCbCrSubsampleRatio422,
}

// A Device holds the state of a connection to a video device.
// Each Device can have at most one stream running.
type Device struct {
	path string
	f    *os.File
	wg   sync.WaitGroup
	ch   chan image.Image
}

type FrameFormat struct {
	Format        Format
	Width, Height int
	rect          image.Rectangle
}

func Open(path string) (dev *Device, err error) {
	dev = &Device{path: path}
	dev.f, err = os.Open(dev.path)
	return
}

// Close closes the underlying file handle to the V4L
// device. It stops any streams in progress, and waits
// for any goroutines to exit.
func (dev *Device) Close() {
	dev.f.Close()
	dev.f = nil

	// This is just to be sure that if the goroutine
	// does not exit, the user becomes aware (because
	// Close() hangs).
	dev.wg.Wait()
}

// Stream configures the device according to the provided FrameFormat.
// Stream returns a channel of Images. The channel is buffered
// so that if the consumer does not consume new images, new ones are
// lost. FrameFormat is validated, and may result in Stream
// returning an error if the frame format is not supported.
//
// It is an error to call Stream on a Device more than once.
//
// Stream starts a goroutine to collect frames from the Device.
// The goroutine exits when Close is called on the Device.
func (dev *Device) Stream(ff FrameFormat) (chan image.Image, error) {
	if dev.ch != nil {
		return nil, fmt.Errorf("A stream is already running on this device.")
	}
	if dev.f == nil {
		return nil, fmt.Errorf("Device is not open.")
	}

	ff.rect = image.Rect(0, 0, ff.Width, ff.Height)
	subsample, ok := ssMap[ff.Format]
	if !ok {
		return nil, fmt.Errorf("Frame format not supported.")
	}

	dev.ch = make(chan image.Image, 1)

	dev.wg.Add(1)
	go func() {
		buf := make([]byte, 0)
		for {
			_, err := dev.f.Read(buf)
			if err != nil {
				break
			}
			dev.ch <- image.NewYCbCr(ff.rect, subsample)
		}
		// Shutdown
		close(dev.ch)
		dev.wg.Done()
	}()

	return dev.ch, nil
}