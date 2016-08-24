package ui

import (
	"runtime"
	"github.com/go-gl/glfw/v3.0/glfw"
	"github.com/go-gl/gl/v2.1/gl"
	"errors"
	"github.com/bionicrm/controlifx"
	_ "image/png"
	"os"
	"image"
	"image/draw"
	"time"
	"sync"
	"math"
)

const (
	Title  = "Emulifx"
	Width  = 512
	Height = 512

	FastestBrightnessChangeDuration = 350*1e6
)

type (
	PowerAction struct {
		On       bool
		Duration uint32
	}

	ColorAction struct {
		Color    controlifx.HSBK
		Duration uint32
	}

	LabelAction controlifx.Label
)

func init() {
	runtime.LockOSThread()
}

func ShowWindow(label, group string, stopCh <-chan interface{}, actionCh <-chan interface{}) error {
	if ok := glfw.Init(); !ok {
		return errors.New("error initializing GLFW")
	}
	defer glfw.Terminate()

	// Configure window.
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	win, err := glfw.CreateWindow(Width, Height, Title +" (off)", nil, nil)
	if err != nil {
		return err
	}

	win.SetKeyCallback(func(w *glfw.Window, key glfw.Key, _ int, action glfw.Action, _ glfw.ModifierKey) {
		if key == glfw.KeyEscape && action == glfw.Press {
			w.SetShouldClose(true)
		}
	})

	win.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return err
	}

	var (
		colorMutex sync.Mutex
		lastBrightness uint16 = 0xffff
		power bool
		hCurrent, sCurrent, bCurrent, kCurrent, hStart, sStart, bStart, kStart, hEnd, sEnd, bEnd, kEnd uint16
		durationStart, duration, bDuration int64

		updateTitle = func() {
			str := Title +": "+label+"@"+group+" ("

			if power {
				str += "on)"
			} else {
				str += "off)"
			}

			win.SetTitle(str)
		}
	)

	// Initialize saturation and Kelvin.
	kCurrent = 2500
	kStart = kCurrent
	kEnd = kCurrent

	updateTitle()

	go func() {
		for {
			select {
			case action := <-actionCh:
				switch action.(type) {
				case PowerAction:
					powerAction := action.(PowerAction)
					power = powerAction.On
					now := time.Now()

					colorMutex.Lock()
					durationStart = now.UnixNano()
					bDuration = int64(math.Max(FastestBrightnessChangeDuration, float64(powerAction.Duration)*1e6))

					if power {
						bStart = bCurrent
						bEnd = lastBrightness
					} else {
						lastBrightness = bCurrent
						bStart = bCurrent
						bEnd = 0
					}
					colorMutex.Unlock()

					updateTitle()
				case ColorAction:
					colorAction := action.(ColorAction)
					color := colorAction.Color
					now := time.Now()

					colorMutex.Lock()
					hStart = hCurrent
					sStart = sCurrent
					bStart = bCurrent
					kStart = kCurrent
					hEnd = color.Hue
					sEnd = color.Saturation
					bEnd = color.Brightness
					kEnd = color.Kelvin

					durationStart = now.UnixNano()
					duration = int64(colorAction.Duration)*1e6
					bDuration = duration

					colorMutex.Unlock()
				case LabelAction:
					label = string(action.(LabelAction))

					updateTitle()
				}
			case <-stopCh:
				win.SetShouldClose(true)
			}
		}
	}()

	tex, err := newTexture("ui/lifx.png")
	if err != nil {
		return err
	}
	defer gl.DeleteTextures(1, &tex)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	for !win.ShouldClose() {
		now := time.Now().UnixNano()

		colorMutex.Lock()
		if now < durationStart+duration {
			hCurrent = lerp(durationStart, duration, now, hStart, int32(hEnd)-int32(hStart))
			sCurrent = lerp(durationStart, duration, now, sStart, int32(sEnd)-int32(sStart))
			kCurrent = lerp(durationStart, duration, now, kStart, int32(kEnd)-int32(kStart))
		} else {
			hCurrent = hEnd
			sCurrent = sEnd
			kCurrent = kEnd
		}

		if now < durationStart+bDuration {
			bCurrent = lerp(durationStart, bDuration, now, bStart, int32(bEnd)-int32(bStart))
		} else {
			bCurrent = bEnd
		}

		setColor(hCurrent, sCurrent, bCurrent, kCurrent)
		colorMutex.Unlock()
		gl.Clear(gl.COLOR_BUFFER_BIT)

		// Draw image.
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.Begin(gl.QUADS)
		gl.Normal3f(0, 0, 1)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(-0.5, 0.5, 1)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(0.5, 0.5, 1)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(0.5, -0.5, 1)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(-0.5, -0.5, 1)
		gl.End()

		win.SwapBuffers()
		glfw.PollEvents()
	}

	return nil
}

// Use Kelvin.
func setColor(h, s, b, k uint16) {
	red, green, blue := hslToRgb(h, s, b)

	gl.ClearColor(red, green, blue, 1)
}

func lerp(durationStart, duration, now int64, vStart uint16, vChange int32) uint16 {
	return uint16(float32(now-durationStart)/float32(duration)*float32(vChange)+float32(vStart))
}

func hslToRgb(hI, sI, lI uint16) (r, g, b float32) {
	h := float32(hI)/0xffff
	s := float32(sI)/0xffff
	l := float32(lI)/0xffff

	if s == 0 {
		r = l
		g = l
		b = l
	} else {
		hueToRgb := func(p, q, t float32) float32 {
			if t < 0 {
				t += 1
			}
			if t > 1 {
				t -= 1
			}
			if t < 1/6.0 {
				return p+(q-p)*6*t
			}
			if t < 1/2.0 {
				return q
			}
			if t < 2/3.0 {
				return p+(q-p)*(2/3.0-t)*6
			}
			return p
		}

		var q float32
		if l < 0.5 {
			q = l*(1+s)
		} else {
			q = l+s-l*s
		}
		p := 2*l-q
		r = hueToRgb(p, q, h+1/3.0)
		g = hueToRgb(p, q, h)
		b = hueToRgb(p, q, h-1/3.0)
	}

	return
}

func newTexture(file string) (uint32, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}

	img, _, err := image.Decode(f)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return 0, errors.New("unsupported stride")
	}

	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	var tex uint32
	gl.Enable(gl.TEXTURE_2D)
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_BORDER)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_BORDER)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	return tex, nil
}
