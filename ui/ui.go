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

func init() {
	runtime.LockOSThread()
}

func ShowWindow(stopCh <-chan interface{}, colorCh <-chan controlifx.HSBK) error {
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

	var hsbk controlifx.HSBK

	go func() {
		for {
			select {
			case hsbk = <-colorCh:
			case <-stopCh:
				win.SetShouldClose(true)
			}
		}
	}()

	for !win.ShouldClose() {
		// TODO: use hsbk
		gl.ClearColor(.5, .5, .25, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		win.SwapBuffers()
		glfw.PollEvents()
	}

	return nil
}
