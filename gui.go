package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var real_path = ""

func initGui() {

	a := app.New()
	w := a.NewWindow("Local GIT Contributions Visualizer")
	//Set Options
	w.Resize(fyne.Size{Width: 1000, Height: 650})
	w.CenterOnScreen()

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter your Git email address: ")

	path := widget.NewLabel("")

	content := container.NewVBox(
		input,
		widget.NewButton("Go Get Directory!", func() {
			chooseDirectory(w, path)
		}), widget.NewButton("Process", func() {
			log.Println("Email is :", input.Text)
			log.Println("Path is :", real_path)
		}))

	w.SetContent(content)

	w.ShowAndRun()
}

func chooseDirectory(w fyne.Window, h *widget.Label) {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		save_dir := "NoPathYet!"
		if err != nil {
			return
		}
		if dir != nil {
			save_dir = dir.Path() // here value of save_dir shall be updated!
			real_path = save_dir
		}
		h.SetText(save_dir)

	}, w)
}
