package dialog

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"
)

type textWidget interface {
	fyne.Widget
	SetText(string)
}

type fileDialog struct {
	fileName   textWidget
	open       *widget.Button
	breadcrumb *widget.Box
	files      *fyne.Container
	parent     fyne.Window

	win      *widget.PopUp
	selected *fileDialogItem
	callback func(string)
	dir      string
	save     bool
}

func (f *fileDialog) makeUI() fyne.CanvasObject {
	if f.save {
		saveName := widget.NewEntry()
		saveName.OnChanged = func(s string) {
			if s == "" {
				f.open.Disable()
			} else {
				f.open.Enable()
			}
		}
		f.fileName = saveName
	} else {
		f.fileName = widget.NewLabel("")
	}

	label := "Open"
	if f.save {
		label = "Save"
	}
	f.open = widget.NewButton(label, func() {
		if f.callback == nil {
			f.win.Hide()
			return
		}

		if f.save {
			name := f.fileName.(*widget.Entry).Text
			path := filepath.Join(f.dir, name)

			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				f.win.Hide()
				f.callback(path)
				return
			} else if info.IsDir() {
				ShowInformation("Cannot overwrite",
					"Files cannot replace a directory,\ncheck the file name and try again", f.parent)
				return
			}

			ShowConfirm("Overwrite?", "Are you sure you want to overwrite the file\n"+name+"?",
				func(ok bool) {
					if !ok {
						f.callback("")
						return
					}

					f.callback(path)
					f.win.Hide()
				}, f.parent)
		} else if f.selected != nil {
			f.win.Hide()
			f.callback(f.selected.path)
		}
	})
	f.open.Style = widget.PrimaryButton
	f.open.Disable()
	buttons := widget.NewHBox(
		widget.NewButton("Cancel", func() {
			f.win.Hide()
			if f.callback != nil {
				f.callback("")
			}
		}),
		f.open)
	footer := fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, nil, nil, buttons),
		buttons, f.fileName)

	f.files = fyne.NewContainerWithLayout(layout.NewFixedGridLayout(fyne.NewSize(fileIconCellWidth,
		fileIconSize+theme.Padding()+fileTextSize)),
	)
	fileScroll := widget.NewScrollContainer(f.files)
	verticalExtra := int(float64(fileIconSize) * 0.25)
	fileScroll.SetMinSize(fyne.NewSize(fileIconCellWidth*4+theme.Padding()*3,
		(fileIconSize+fileTextSize)*2+theme.Padding()*4+verticalExtra))

	f.breadcrumb = widget.NewHBox()
	scrollBread := widget.NewScrollContainer(f.breadcrumb)
	body := fyne.NewContainerWithLayout(layout.NewBorderLayout(scrollBread, nil, nil, nil),
		scrollBread, fileScroll)
	header := widget.NewLabelWithStyle(label+" File", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	favourites := widget.NewGroup("Favourites", f.loadFavourites()...)
	return fyne.NewContainerWithLayout(layout.NewBorderLayout(header, footer, favourites, nil),
		favourites, header, footer, body)
}

func (f *fileDialog) loadFavourites() []fyne.CanvasObject {
	home, _ := os.UserHomeDir()
	places := []fyne.CanvasObject{
		widget.NewButton("Home", func() {
			f.setDirectory(home)
		}),
		widget.NewButton("Documents", func() {
			f.setDirectory(filepath.Join(home, "Documents"))
		}),
		widget.NewButton("Downloads", func() {
			f.setDirectory(filepath.Join(home, "Downloads"))
		}),
	}

	places = append(places, f.loadPlaces()...)
	return places
}

func (f *fileDialog) refreshDir(dir string) {
	f.files.Objects = nil

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fyne.LogError("Unable to read path "+dir, err)
		return
	}

	var icons []fyne.CanvasObject
	parent := filepath.Dir(dir)
	if parent != dir {
		icons = append(icons, f.newFileIcon(theme.FolderOpenIcon(), filepath.Dir(dir)))
	}
	for _, file := range files {
		if isHidden(file.Name(), dir) {
			continue
		}

		itemPath := filepath.Join(dir, file.Name())
		if file.IsDir() {
			icons = append(icons, f.newFileIcon(theme.FolderIcon(), itemPath))
		} else {
			icons = append(icons, f.newFileIcon(theme.FileIcon(), itemPath))
		}
	}

	f.files.Objects = icons
	f.files.Refresh()
}

func (f *fileDialog) setDirectory(dir string) {
	f.setSelected(nil)
	f.dir = dir

	f.breadcrumb.Children = nil
	buildDir := filepath.VolumeName(dir)
	for i, d := range strings.Split(dir, string(filepath.Separator)) {
		if d == "" {
			if i > 0 { // what we get if we split "/"
				break
			}
			buildDir = "/"
			d = "/"
		} else if i > 0 {
			buildDir = filepath.Join(buildDir, d)
		} else {
			d = buildDir
			buildDir = d + string(os.PathSeparator)
		}

		newDir := buildDir
		f.breadcrumb.Append(
			widget.NewButton(d, func() {
				f.setDirectory(newDir)
			}),
		)
	}

	f.refreshDir(dir)
}

func (f *fileDialog) setSelected(file *fileDialogItem) {
	if f.selected != nil {
		f.selected.isCurrent = false
		f.selected.Refresh()
	}
	if file != nil && file.isDirectory() {
		f.setDirectory(file.path)
		return
	}
	f.selected = file

	if file == nil || file.path == "" {
		f.fileName.SetText("")
		f.open.Disable()
	} else {
		file.isCurrent = true
		f.fileName.SetText(filepath.Base(file.path))
		f.open.Enable()
	}
}

func showFileDialog(save bool, callback func(string), parent fyne.Window) {
	if fileOSOverride(save, callback, parent) {
		return
	}
	d := &fileDialog{callback: callback, save: save, parent: parent}
	ui := d.makeUI()
	dir, err := os.UserHomeDir()
	if err != nil {
		fyne.LogError("Could not load user home dir", err)
		dir, _ = os.Getwd() //fallback
	}
	d.setDirectory(dir)

	d.win = widget.NewModalPopUp(ui, parent.Canvas())
	d.win.Show()
}

// ShowFileOpen shows a file dialog allowing the user to choose a file to open.
// The dialog will appear over the window specified.
func ShowFileOpen(callback func(string), parent fyne.Window) {
	showFileDialog(false, callback, parent)
}

// ShowFileSave shows a file dialog allowing the user to choose a file to save to (new or overwrite).
// If the user chooses an existing file they will be asked if they are sure.
// The dialog will appear over the window specified.
func ShowFileSave(callback func(string), parent fyne.Window) {
	showFileDialog(true, callback, parent)
}
