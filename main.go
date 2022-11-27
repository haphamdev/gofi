package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/tools/godoc/util"
	"golang.org/x/tools/godoc/vfs"
)

//Keycodes
const (
	J rune = 106
	K      = 107
	Q      = 113
)

type Item struct {
	Title       string
	Description string
	Footer      string
}

/**
* Represents for an Application
* Including:
* - all tview primitives of the app
* - all data needed by the app
* - some channels to comminicate between goroutines
 */
type Application struct {
	application        *tview.Application
	listView           *tview.List
	headerView         *tview.TextView
	footerView         *tview.TextView
	descriptionView    *tview.TextArea
	items              []Item
	selectedItemIndex  int
	itemAdded          chan Item      // new item is added via this channel
	stopReceiveNewItem chan bool      // signal other goroutines to quit
	waitgroup          sync.WaitGroup // wait until all goroutines complete
}

func newApplication() *Application {
	app := Application{
		application:        tview.NewApplication(),
		listView:           tview.NewList(),
		headerView:         tview.NewTextView(),
		footerView:         tview.NewTextView(),
		descriptionView:    tview.NewTextArea(),
		itemAdded:          make(chan Item, 100),
		stopReceiveNewItem: make(chan bool, 10),
	}

	// Redraw the other views when list selected item is changed
	app.listView.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		app.selectedItemIndex = index
		item := app.items[index]
		log.Println("Selected item ", item.Title)
		app.headerView.SetText(item.Title)
		app.descriptionView.SetText(item.Description, false)
		app.footerView.SetText(fmt.Sprintf("%d/%d: %s", index+1, len(app.items), item.Footer))
	})

	app.listView.SetMouseCapture(
		func(mouseAction tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			// Set focus on the list view when clicking on it
			// Without this, the list item will be selected, but the listview is not focused
			app.application.SetFocus(app.listView)
			return mouseAction, event
		},
	)

	app.application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Before quiting, send quit signal to stop the coroutine and stop receiving new items
		if event.Key() == tcell.KeyCtrlC {
			log.Println("Pressed Ctrl+C, stopping app...")
			app.stopReceiveNewItem <- true
		}

		// Use j and k to navigate through the list
		if app.application.GetFocus() == app.listView {
			if event.Rune() == J {
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			} else if event.Rune() == K {
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			} else if event.Rune() == Q {
				log.Println("Pressed Q, stopping app...")
				app.stopReceiveNewItem <- true
				return tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
			}
		}

		return event
	})

	return &app
}

/**
* Start the application in the main goroutine.
* Launch a new goroutine to receive the added items.
* This new goroutine will stop when receiving a signal in the app.quit channel
 */
func (app *Application) start() {
	log.Println("Starting application...")
	app.listView.SetBorder(true)
	app.descriptionView.SetBorder(true)
	app.headerView.SetBorder(true)
	app.footerView.SetBorder(true)

	app.waitgroup.Add(1)
	// a new goroutine to receive new items from app.itemAdded channel
	go func() {
	NEW_ITEM_LOOP:
		for {
			select {
			case stop := <-app.stopReceiveNewItem:
				if stop {
					close(app.itemAdded)
					app.waitgroup.Done()
					break NEW_ITEM_LOOP
				}
			case newItem := <-app.itemAdded:
				log.Println("Receiving newly added item: ", newItem.Title)
				app.addItem(&newItem)
			default:
				continue
			}
		}

		log.Println("No longer receive new items")
	}()

	flex := tview.NewFlex().
		AddItem(app.listView, 30, 0, true).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(app.headerView, 3, 0, false).
				AddItem(app.descriptionView, 0, 1, false).
				AddItem(app.footerView, 3, 0, false),
			0, 7, false,
		)

		// start tview application
	if err := app.application.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}

	log.Println("Waiting until no longer receiving new item...")
	app.waitgroup.Wait()
	log.Println("Stopped")
}

func (app *Application) addItem(item *Item) {
	app.items = append(app.items, *item)
	app.application.QueueUpdateDraw(func() {
		app.listView.AddItem(item.Title, "", 0, nil)
		// Redraw footer to because the total item is changed
		app.footerView.SetText(fmt.Sprintf("%d/%d: %s", app.selectedItemIndex+1, len(app.items), item.Footer))
	})
}

/**
* Read the directory and create one Item for each file/subdir found
* The created items will be sent to itemAddedChannel
*/
func scanDirectory(itemAddedChannel chan<- Item) {
	var path string
	if len(os.Args) == 1 {
		log.Println("No path, using current working directory")
		currentDir, err := os.Getwd()

		if err != nil {
			log.Println("Unable to get current directory. ", err)
			return
		}

		log.Println("Current directory: ", currentDir)
		path = currentDir
	} else {
		path = os.Args[1]
		log.Println("Path: ", path)
	}

	files, err := ioutil.ReadDir(path)

	if err != nil {
		log.Println("Unable to read dir: ", path, err)
		return
	}

	for _, file := range files {
		log.Println("Adding ", file.Name())
		fullPath := fmt.Sprintf("%s/%s", path, file.Name())
		fileStat, err := os.Stat(fullPath)

		if err != nil {
			log.Printf("Unable to get stat of '%s'. %s", file.Name(), err)
			continue
		}

		isDir := "No"
		if fileStat.IsDir() {
			isDir = "Yes"
		}

		fileFormat := "Bin"
		if util.IsTextFile(vfs.OS("/"), fullPath) {
			fileFormat = "Text"
		}

		newItem := Item{
			Title: file.Name(),
			Description: fmt.Sprintf(
				"File size: %d\nParent dir: %s\nFile mode: %s\nDirectory: %s\nFile format: %s",
				fileStat.Size(),
				path,
				fileStat.Mode(),
				isDir,
				fileFormat,
			),
			Footer: fullPath,
		}

		itemAddedChannel <- newItem
	}
}

func main() {
	initLogger()
	application := newApplication()
	go scanDirectory(application.itemAdded)
	application.start()
}

func initLogger() {
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)
}

