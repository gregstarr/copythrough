package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"golang.design/x/clipboard"
	"log"
	"os"
	"path"
	"time"
)

type Message struct {
	Origin string
	Format clipboard.Format
	Data   []byte
}

func (m Message) String() string {
	switch m.Format {
	case clipboard.FmtText:
		return fmt.Sprintf("%s: %s", m.Origin, string(m.Data))
	case clipboard.FmtImage:
		return fmt.Sprintf("%s: image", m.Origin)
	default:
		return ""
	}
}

func main() {
	dir := os.Args[1]
	copyfile := path.Join(dir, ".COPYTHROUGH")

	if err := clipboard.Init(); err != nil {
		log.Fatal(err)
	}

	host, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	var rxBuffer bytes.Buffer
	dec := gob.NewDecoder(&rxBuffer)
	var receivedMessage Message

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					data, err := os.ReadFile(copyfile)
					if err != nil {
						log.Fatal(err)
					}
					log.Println(event.Name)
					rxBuffer.Write(data)
					dec.Decode(&receivedMessage)
					rxBuffer.Reset()
					log.Println(receivedMessage.String())
					if receivedMessage.Origin == host {
						continue
					}
					clipboard.Write(receivedMessage.Format, receivedMessage.Data)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(copyfile)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		laststatus := false
		_, err := os.Stat(copyfile)
		if err == nil {
			laststatus = false
		}
		for {
			_, err := os.Stat(copyfile)
			if err != nil && errors.Is(err, os.ErrNotExist) {
				// file not exist
				laststatus = false
			}
			if err == nil && laststatus == false {
				watcher.Events <- fsnotify.Event{"USB Plugged In", fsnotify.Write}
			}
			if err == nil {
				laststatus = true
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	watcher.Events <- fsnotify.Event{"Greg", 0}

	textChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	imgChan := clipboard.Watch(context.Background(), clipboard.FmtImage)
	var txBuffer bytes.Buffer
	enc := gob.NewEncoder(&txBuffer)
	for {
		select {
		case data := <-textChan:
			msg := Message{host, clipboard.FmtText, data}
			enc.Encode(msg)
			if err := os.WriteFile(copyfile, txBuffer.Bytes(), 0644); err != nil {
				log.Fatal(err)
			}
			txBuffer.Reset()
			log.Println(msg.String())
		case data := <-imgChan:
			msg := Message{host, clipboard.FmtImage, data}
			enc.Encode(msg)
			if err := os.WriteFile(copyfile, txBuffer.Bytes(), 0644); err != nil {
				log.Fatal(err)
			}
			txBuffer.Reset()
			log.Println(msg.String())
		}
	}
}
