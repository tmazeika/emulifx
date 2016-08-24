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
		hChange, sChange, bChange, kChange int32
		farHChange, hChangeLeft bool
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
	kCurrent = 3500
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

					bChange = int32(bEnd)-int32(bStart)
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
					sChange = int32(sEnd)-int32(sStart)
					bChange = int32(bEnd)-int32(bStart)
					kChange = int32(kEnd)-int32(kStart)

					// The hue's change takes the shortest distance.
					if farHChange = difference(hStart, hEnd) > 0xffff/2; farHChange {
						start := int32(hStart)
						end := int32(hEnd)

						if hChangeLeft = end-start > 0; hChangeLeft {
							// Moving right, switch to left.
							hChange = -0xffff-start+end
						} else {
							// Moving left, switch to right.
							hChange = 0xffff-start+end
						}
					} else {
						hChange = int32(hEnd)-int32(hStart)
					}

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
			if farHChange {
				hCurrentI := lerp(durationStart, duration, now, hStart, hChange)

				if hChangeLeft {
					hCurrent = uint16(0xffff+hCurrentI%0xffff)
				} else {
					hCurrent = uint16(hCurrentI)
				}
			} else {
				hCurrent = uint16(lerp(durationStart, duration, now, hStart, hChange))
			}

			sCurrent = uint16(lerp(durationStart, duration, now, sStart, sChange))
			kCurrent = uint16(lerp(durationStart, duration, now, kStart, kChange))
		} else {
			hCurrent = hEnd
			sCurrent = sEnd
			kCurrent = kEnd
		}

		if now < durationStart+bDuration {
			bCurrent = uint16(lerp(durationStart, bDuration, now, bStart, bChange))
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

func difference(a, b uint16) int32 {
	diff := int32(a)-int32(b)

	if diff < 0 {
		return -diff
	} else {
		return diff
	}
}

func setColor(h, s, b, k uint16) {
	red, green, blue := hslToRgb(h, s, b)
	kRed, kGreen, kBlue := kToRgb(float32(k))

	gl.ClearColor(red*kRed, green*kGreen, blue*kBlue, 1)
}

func lerp(durationStart, duration, now int64, vStart uint16, vChange int32) int32 {
	return int32(float32(now-durationStart)/float32(duration)*float32(vChange)+float32(vStart))
}

// Credit to http://www.tannerhelland.com/4435/convert-temperature-rgb-algorithm-code/.
func kToRgb(k float32) (r, g, b float32) {
	k /= 100

	// Red.
	if k <= 66 {
		r = 255
	} else {
		r = k-60
		r = 329.698727446*float32(math.Pow(float64(r), -0.1332047592))

		if r < 0 {
			r = 0
		} else if r > 255 {
			r = 255
		}
	}

	// Green.
	if k <= 66 {
		g = k
		g = 99.4708025861*float32(math.Log(float64(g)))-161.1195681661

		if g < 0 {
			g = 0
		} else if g > 255 {
			g = 255
		}
	} else {
		g = k-60
		g = 288.1221695283*float32(math.Pow(float64(g), -0.0755148492))

		if g < 0 {
			g = 0
		} else if g > 255 {
			g = 255
		}
	}

	// Blue.
	if k >= 66 {
		b = 255
	} else {
		if k <= 19 {
			b = 0
		} else {
			b = k-10
			b = 138.5177312231*float32(math.Log(float64(b)))-305.0447927307

			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}
		}
	}

	return r/255, g/255, b/255
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
