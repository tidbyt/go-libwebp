package webp

/*
#cgo LDFLAGS: -lwebpmux

#include <stdlib.h>
#include <string.h>
#include <webp/encode.h>
#include <webp/mux.h>
#include <stdbool.h>

int writeWebP(uint8_t *, size_t, struct WebPPicture *);

static WebPPicture *calloc_WebPPicture(void)
{
    return calloc(sizeof(WebPPicture), 1);
}

static void free_WebPPicture(WebPPicture *webpPicture)
{
    free(webpPicture);
}

static int EncodeWrapper(uint8_t *imgs[], int numImgs, int stride, int width, int height, int duration, WebPData *data, int rgba)
{
    int errNo = 0;
    int timestamp = 0;
    WebPAnimEncoder *ae;

    // Create animation encoder.
    WebPAnimEncoderOptions opts;
    errNo = WebPAnimEncoderOptionsInit(&opts);
    if (errNo != 0)
    {
        goto cleanup;
    }
    opts.kmin = 0;
    opts.kmax = 0;
    ae = WebPAnimEncoderNew(width, height, &opts);
    if (ae == NULL)
    {
        errNo = 7;
        goto cleanup;
    }

    // Add frames.
    for (int i = 0; i < numImgs; i++)
    {
        WebPPicture *pic = calloc(sizeof(WebPPicture), 1);
        errNo = WebPPictureInit(pic);
        if (errNo != 0)
        {
            goto cleanup;
        }

        pic->use_argb = 1;
        pic->width = width;
        pic->height = height;
        pic->writer = writeWebP;
        if (rgba == 1)
        {
            WebPPictureImportRGBA(pic, imgs[i], stride);
        }
        else
        {
            WebPPictureImportRGB(pic, imgs[i], stride);
        }
        errNo = WebPAnimEncoderAdd(ae, pic, timestamp, NULL);
        timestamp += duration;
        free(pic);
        if (errNo != 0)
        {
            goto cleanup;
        }
    }

    // Add final empty frame.
    WebPAnimEncoderAdd(ae, NULL, duration, NULL);

    // Assemble animation.
    WebPDataInit(data);
    errNo = WebPAnimEncoderAssemble(ae, data);

cleanup:

    // Clean up.
    if (ae != NULL)
    {
        WebPAnimEncoderDelete(ae);
    }

    // Return data.
    return errNo;
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

// AnimationEncoder encodes multiple images into an animated WebP.
type AnimationEncoder struct {
	opts     C.WebPAnimEncoderOptions
	c        *C.WebPAnimEncoder
	duration time.Duration
}

// NewAnimationEncoder initializes a new encoder.
func NewAnimationEncoder(width, height, kmin, kmax int) (*AnimationEncoder, error) {
	ae := &AnimationEncoder{}

	if C.WebPAnimEncoderOptionsInit(&ae.opts) == 0 {
		return nil, errors.New("failed to initialize animation encoder config")
	}
	ae.opts.kmin = C.int(kmin)
	ae.opts.kmax = C.int(kmax)

	ae.c = C.WebPAnimEncoderNew(C.int(width), C.int(height), &ae.opts)
	if ae.c == nil {
		return nil, errors.New("failed to initialize animation encoder")
	}

	return ae, nil
}

// AddFrame adds a frame to the encoder.
func (ae *AnimationEncoder) AddFrame(img image.Image, duration time.Duration) error {
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

	pic.writer = C.WebPWriterFunction(C.writeWebP)

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

	if C.WebPAnimEncoderAdd(ae.c, pic, timestamp, nil) == 0 {
		return fmt.Errorf(
			"Encoding error: %d - %s",
			int(pic.error_code),
			C.GoString(C.WebPAnimEncoderGetError(ae.c)),
		)
	}

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

func EncodeWebPAnimation(images []image.Image, frameDuration time.Duration) ([]byte, error) {
	if len(images) == 0 {
		return []byte{}, nil
	}

	bounds := images[0].Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var stride int
	var rgba int
	pixels := []*C.uint8_t{}
	for _, img := range images {
		switch p := img.(type) {
		case *RGBImage:
			rgba = 0
			stride = p.Stride
			pixels = append(pixels, (*C.uint8_t)(&p.Pix[0]))
		case *image.RGBA:
			rgba = 1
			stride = p.Stride
			pixels = append(pixels, (*C.uint8_t)(&p.Pix[0]))
		case *image.NRGBA:
			rgba = 1
			stride = p.Stride
			pixels = append(pixels, (*C.uint8_t)(&p.Pix[0]))
		default:
			return nil, errors.New("unsupported image type")
		}
	}

	data := &C.WebPData{}
	if C.EncodeWrapper(
		pixels,
		C.int(len(images)),
		C.int(stride),
		C.int(width),
		C.int(height),
		C.int(frameDuration/time.Millisecond),
		data,
		C.int(rgba),
	) != 0 {
		return nil, errors.New("Error encoding WebP animation")
	}

	return C.GoBytes(
		unsafe.Pointer(data.bytes),
		C.int(int(data.size)),
	), nil
}
