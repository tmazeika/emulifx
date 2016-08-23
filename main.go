package main

import (
	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/glfw/v3.0/glfw"
	"log"
	"runtime"
	"time"
	"math"
	"fmt"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	fmt.Print("starting... ")

	if ok := glfw.Init(); !ok {
		log.Fatalln("error initializing GLFW")
	}
	defer glfw.Terminate()

	win, err := glfw.CreateWindow(500, 500, "Emulifx", nil, nil)
	if err != nil {
		log.Fatalln(err)
	}

	win.SetKeyCallback(func(w *glfw.Window, key glfw.Key, _ int, action glfw.Action, _ glfw.ModifierKey) {
		if key == glfw.KeyEscape && action == glfw.Press {
			fmt.Print("closing... ")
			w.SetShouldClose(true)
		}
	})

	win.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatalln(err)
	}

	var frames int

	// FPS counter.
	go func() {
		now := time.Now()
		lastFpsAt := now

		for {
			now = time.Now()

			if now.Sub(lastFpsAt).Nanoseconds() >= 1e9 {
				fmt.Println("FPS:", frames)
				lastFpsAt = now
				frames = 0
			}
		}
	}()

	fmt.Println("done")

	for !win.ShouldClose() {
		r := math.Abs(math.Sin(float64(time.Now().UnixNano())/1.0e9))
		g := math.Abs(math.Sin(float64(time.Now().UnixNano())/1.0e9+2))
		b := math.Abs(math.Sin(float64(time.Now().UnixNano())/1.0e9+4))
		gl.ClearColor(float32(r), float32(g), float32(b), 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		win.SwapBuffers()
		glfw.PollEvents()

		frames++
	}

	fmt.Println("done")
}
