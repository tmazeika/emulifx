package ui

import (
	"runtime"
	"github.com/go-gl/glfw/v3.0/glfw"
	"github.com/go-gl/gl/v2.1/gl"
	"errors"
	"github.com/bionicrm/controlifx"
)

const (
	title  = "Emulifx"
	width  = 500
	height = 500
)

type (
	PowerAction struct {
		On       bool
		Duration uint16
	}

	ColorAction struct {
		Color    controlifx.HSBK
		Duration uint16
	}
)

func init() {
	runtime.LockOSThread()
}

func ShowWindow(stopCh <-chan interface{}, powerCh <-chan PowerAction, colorCh <-chan ColorAction) error {
	if ok := glfw.Init(); !ok {
		return errors.New("error initializing GLFW")
	}
	defer glfw.Terminate()

	win, err := glfw.CreateWindow(width, height, title, nil, nil)
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
	)

	go func() {
		for {
			select {
			case powerAction := <-powerCh:
				if powerAction.On {
					hsbk.Brightness = lastBrightness
				} else {
					if hsbk.Brightness > 0 {
						lastBrightness = hsbk.Brightness
					}

					hsbk.Brightness = 0
				}
			case colorAction := <-colorCh:
				hsbk = colorAction.Color
				lastBrightness = hsbk.Brightness
			case <-stopCh:
				win.SetShouldClose(true)
			}

			r, g, b = hslToRgb(hsbk.Hue, hsbk.Saturation, hsbk.Brightness)
		}
	}()

	for !win.ShouldClose() {
		// TODO: implement Kelvin and duration
		gl.ClearColor(r, g, b, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

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
