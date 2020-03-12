package webp

/*
#include <stdlib.h>
#include <string.h>
#include <webp/encode.h>
#include <webp/mux.h>

int writeWebP(uint8_t*, size_t, struct WebPPicture*);

static WebPPicture *calloc_WebPPicture(void) {
	return calloc(sizeof(WebPPicture), 1);
}

static void free_WebPPicture(WebPPicture* webpPicture) {
	free(webpPicture);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"time"
	"unsafe"
)

func Hello() {
	fmt.Println("Hello, World!")
	enc_version := C.WebPGetEncoderVersion()
	mux_version := C.WebPGetMuxVersion()
	fmt.Printf("WebP Encoder version: %d.%d.%d\nWebP Mux version: %d.%d.%d\n",
		(enc_version>>16)&0xff, (enc_version>>8)&0xff,
		enc_version&0xff, (mux_version>>16)&0xff,
		(mux_version>>8)&0xff, mux_version&0xff)
}

// AnimationEncoder encodes multiple images into an animated WebP.
type AnimationEncoder struct {
	opts     C.WebPAnimEncoderOptions
	c        *C.WebPAnimEncoder
	duration time.Duration
}

// NewAnimationEncoder initializes a new encoder.
func NewAnimationEncoder(width, height, kmin, kmax int, minimize_size, allow_mixed bool) (*AnimationEncoder, error) {
	ae := &AnimationEncoder{}

	if C.WebPAnimEncoderOptionsInit(&ae.opts) == 0 {
		return nil, errors.New("failed to initialize animation encoder config")
	}
	ae.opts.kmin = C.int(kmin)
	ae.opts.kmax = C.int(kmax)
	ae.opts.minimize_size = C.int(0)
	if minimize_size {
		ae.opts.minimize_size = C.int(1)
	}
	ae.opts.allow_mixed = C.int(0)
	if minimize_size {
		ae.opts.allow_mixed = C.int(1)
	}

	ae.c = C.WebPAnimEncoderNew(C.int(width), C.int(height), &ae.opts)
	if ae.c == nil {
		return nil, errors.New("failed to initialize animation encoder")
	}

	return ae, nil
}

// AddFrame adds a frame to the encoder.
func (ae *AnimationEncoder) AddFrame(img image.Image, duration time.Duration, lossless bool) error {
	pic := C.calloc_WebPPicture()
	if pic == nil {
		return errors.New("Could not allocate webp picture")
	}
	defer C.free_WebPPicture(pic)

	if C.WebPPictureInit(pic) == 0 {
		return errors.New("Could not initialize webp picture")
	}
	defer C.WebPPictureFree(pic)

	pic.use_argb = 1

	pic.width = C.int(img.Bounds().Dx())
	pic.height = C.int(img.Bounds().Dy())

	switch p := img.(type) {
	case *RGBImage:
		C.WebPPictureImportRGB(pic, (*C.uint8_t)(&p.Pix[0]), C.int(p.Stride))
	case *image.RGBA:
		C.WebPPictureImportRGBA(pic, (*C.uint8_t)(&p.Pix[0]), C.int(p.Stride))
	case *image.NRGBA:
		C.WebPPictureImportRGBA(pic, (*C.uint8_t)(&p.Pix[0]), C.int(p.Stride))
	default:
		return errors.New("unsupported image type")
	}

	timestamp := C.int(ae.duration / time.Millisecond)
	ae.duration += duration

	var config C.WebPConfig
	C.WebPConfigInit(&config)
	if lossless {
		config.lossless = C.int(1)
	} else {
		config.lossless = C.int(0)
	}

	if C.WebPAnimEncoderAdd(ae.c, pic, timestamp, &config) == 0 {
		return fmt.Errorf(
			"Encoding error: %d - %s",
			int(pic.error_code),
			C.GoString(C.WebPAnimEncoderGetError(ae.c)),
		)
	}
	// Stuff to try:
	//  - Pass a WebPConfig to AnimEncoderAdd as last param, and tweak it
	//  - asdf
	return nil
}

// Assemble assembles all frames into animated WebP.
func (ae *AnimationEncoder) Assemble() ([]byte, error) {
	// add final empty frame
	if C.WebPAnimEncoderAdd(ae.c, nil, C.int(ae.duration/time.Millisecond), nil) == 0 {
		return nil, errors.New("Couldn't add final empty frame")
	}

	data := &C.WebPData{}
	C.WebPDataInit(data)

	if C.WebPAnimEncoderAssemble(ae.c, data) == 0 {
		return nil, errors.New("Error assembling animation")
	}

	return C.GoBytes(
		unsafe.Pointer(data.bytes),
		C.int(int(data.size)),
	), nil
}

// Close deletes the encoder and frees resources.
func (ae *AnimationEncoder) Close() {
	C.WebPAnimEncoderDelete(ae.c)
}
