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
)

const (
	title  = "Emulifx"
	width  = 512
	height = 512
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

	win, err := glfw.CreateWindow(width, height, title+" (off)", nil, nil)
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
		lastBrightness uint16 = 0xffff
		hsbk = controlifx.HSBK{
			Kelvin:2500,
		}
		r, g, b float32

		updateTitle = func() {
			str := title+": "+label+"@"+group+" ("

			if hsbk.Brightness == 0 {
				str += "off)"
			} else {
				str += "on)"
			}

			win.SetTitle(str)
		}
	)

	updateTitle()

	go func() {
		for {
			select {
			case action := <-actionCh:
				switch action.(type) {
				case PowerAction:
					if action.(PowerAction).On {
						hsbk.Brightness = lastBrightness
					} else {
						if hsbk.Brightness > 0 {
							lastBrightness = hsbk.Brightness
						}

						hsbk.Brightness = 0
					}

					updateTitle()
				case ColorAction:
					color := action.(ColorAction).Color

					if hsbk.Brightness == 0 {
						hsbk = controlifx.HSBK{
							Hue:color.Hue,
							Saturation:color.Saturation,
							Kelvin:color.Kelvin,
						}
					} else {
						hsbk = color
					}

					if color.Brightness > 0 {
						lastBrightness = color.Brightness
					}
				case LabelAction:
					label = string(action.(LabelAction))

					updateTitle()
				}
			case <-stopCh:
				win.SetShouldClose(true)
			}

			r, g, b = hslToRgb(hsbk.Hue, hsbk.Saturation, hsbk.Brightness)
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
		// TODO: implement Kelvin and duration
		gl.ClearColor(r, g, b, 1)
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
