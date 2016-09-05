package ui

import (
	"bytes"
	"errors"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"gopkg.in/lifx-tools/controlifx.v1"
	"image"
	"image/draw"
	_ "image/png"
	"math"
	"runtime"
	"sync"
	"time"
)

const (
	Title  = "Emulifx"
	Width  = 512
	Height = 512

	FastestBrightnessChangeDuration = 350 * 1e6
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
)

func init() {
	runtime.LockOSThread()
}

func ShowWindow(hasColor bool, laddr string, stopCh <-chan interface{}, actionCh <-chan interface{}) error {
	if err := glfw.Init(); err != nil {
		return err
	}
	defer glfw.Terminate()

	// Configure window.
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	win, err := glfw.CreateWindow(Width, Height, Title, nil, nil)
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

		// Power.
		bLast     int32 = 0xffff
		poweredOn bool

		// Current.
		hCurrent, sCurrent, bCurrent, kCurrent,
		// Start.
		hStart, sStart, bStart, kStart,
		// End.
		hEnd, sEnd, bEnd, kEnd,
		// Change.
		hChange, sChange, bChange, kChange int32

		// Duration.
		durationStart, duration, bDurationStart, bDuration int64

		updateTitle = func() {
			str := Title + " - " + laddr + " ("

			if poweredOn {
				str += "on)"
			} else {
				str += "off)"
			}

			win.SetTitle(str)
		}
	)

	// Initialize Kelvin.
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
					poweredOn = powerAction.On
					now := time.Now()

					colorMutex.Lock()
					bDurationStart = now.UnixNano()
					bDuration = int64(math.Max(FastestBrightnessChangeDuration, float64(durationToNano(powerAction.Duration))))

					if poweredOn {
						bStart = bCurrent
						bEnd = bLast
					} else {
						bLast = bCurrent
						bStart = bCurrent
						bEnd = 0
					}

					bChange = bEnd - bStart
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
					hEnd = int32(color.Hue)
					sEnd = int32(color.Saturation)
					bEnd = int32(color.Brightness)
					kEnd = int32(color.Kelvin)
					hChange = hEnd - hStart
					sChange = sEnd - sStart
					bChange = bEnd - bStart
					kChange = kEnd - kStart

					// Hue change takes the shortest
					// distance.
					if abs(hChange) > 0xffff/2 {
						if hChange > 0 {
							hChange -= 0xffff
						} else {
							hChange += 0xffff
						}
					}

					durationStart = now.UnixNano()
					bDurationStart = durationStart
					duration = durationToNano(colorAction.Duration)
					bDuration = duration

					colorMutex.Unlock()
				}
			case <-stopCh:
				win.SetShouldClose(true)
			}
		}
	}()

	// Add LIFX logo.
	tex, err := newTexture("res/lifx.png")
	if err != nil {
		return err
	}
	defer gl.DeleteTextures(1, &tex)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	for !win.ShouldClose() {
		now := time.Now().UnixNano()

		colorMutex.Lock()
		// Hue, saturation, and Kelvin linear interpolation.
		if now < durationStart+duration {
			hCurrent = lerp(durationStart, duration, now, hStart, hChange)
			sCurrent = lerp(durationStart, duration, now, sStart, sChange)
			kCurrent = lerp(durationStart, duration, now, kStart, kChange)
		} else {
			hCurrent = hEnd
			sCurrent = sEnd
			kCurrent = kEnd
		}

		// Brightness linear interpolation.
		if now < bDurationStart+bDuration {
			bCurrent = lerp(bDurationStart, bDuration, now, bStart, bChange)
		} else {
			bCurrent = bEnd
		}

		if hasColor {
			setColor(float32(hCurrent)/0xffff, float32(sCurrent)/0xffff, float32(bCurrent)/0xffff/2, float32(kCurrent))
		} else {
			// Non-color bulbs have no hue or saturation.
			setColor(0, 0, float32(bCurrent)/0xffff, float32(kCurrent))
		}

		colorMutex.Unlock()
		gl.Clear(gl.COLOR_BUFFER_BIT)

		// Draw LIFX logo.
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

func abs(a int32) int32 {
	if a < 0 {
		return -a
	}

	return a
}

func setColor(h, s, b, k float32) {
	red, green, blue := hslToRgb(h, s, b)
	kRed, kGreen, kBlue := kToRgb(k)

	gl.ClearColor(red*kRed*2, green*kGreen*2, blue*kBlue*2, 1)
}

func lerp(durationStart, duration, now int64, vStart, vChange int32) int32 {
	return int32(float32(now-durationStart)/float32(duration)*float32(vChange) + float32(vStart))
}

// Credit to http://www.tannerhelland.com/4435/convert-temperature-rgb-algorithm-code/.
func kToRgb(k float32) (r, g, b float32) {
	k /= 100

	// Red.
	if k <= 66 {
		r = 255
	} else if r = 329.698727446 * float32(math.Pow(float64(k-60), -0.1332047592)); r < 0 {
		r = 0
	} else if r > 255 {
		r = 255
	}

	// Green.
	if k <= 66 {
		if g = 99.4708025861*float32(math.Log(float64(k))) - 161.1195681661; g < 0 {
			g = 0
		} else if g > 255 {
			g = 255
		}
	} else if g = 288.1221695283 * float32(math.Pow(float64(k-60), -0.0755148492)); g < 0 {
		g = 0
	} else if g > 255 {
		g = 255
	}

	// Blue.
	if k >= 66 {
		b = 255
	} else if k <= 19 {
		b = 0
	} else if b = 138.5177312231*float32(math.Log(float64(k-10))) - 305.0447927307; b < 0 {
		b = 0
	} else if b > 255 {
		b = 255
	}

	return r / 255, g / 255, b / 255
}

func hslToRgb(h, s, l float32) (r, g, b float32) {
	if s == 0 {
		r = l
		g = l
		b = l
	} else {
		hueToRgb := func(p, q, t float32) float32 {
			if t < 0 {
				t += 1
			} else if t > 1 {
				t -= 1
			}

			if t < 1/6.0 {
				return p + (q-p)*6*t
			}
			if t < 1/2.0 {
				return q
			}
			if t < 2/3.0 {
				return p + (q-p)*(2/3.0-t)*6
			}
			return p
		}

		var q float32
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRgb(p, q, h+1/3.0)
		g = hueToRgb(p, q, h)
		b = hueToRgb(p, q, h-1/3.0)
	}

	return
}

func durationToNano(d uint32) int64 {
	return int64(d) * 1e6
}

func newTexture(file string) (uint32, error) {
	dataReader := bytes.NewReader(MustAsset(file))

	img, _, err := image.Decode(dataReader)
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
