package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"golang.design/x/clipboard"
	"log"
	"os"
	"path"
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
	copyfile := path.Join(dir, "copythrough")

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
					rxBuffer.Write(data)
					dec.Decode(&receivedMessage)
					rxBuffer.Reset()
					log.Println(receivedMessage)
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

	textChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	imgChan := clipboard.Watch(context.Background(), clipboard.FmtImage)
	var txBuffer bytes.Buffer
	enc := gob.NewEncoder(&txBuffer)
	for {
		select {
		case data := <-textChan:
			enc.Encode(Message{host, clipboard.FmtText, data})
			if err := os.WriteFile(copyfile, txBuffer.Bytes(), 0644); err != nil {
				log.Fatal(err)
			}
			txBuffer.Reset()
			log.Println(receivedMessage)
		case data := <-imgChan:
			enc.Encode(Message{host, clipboard.FmtImage, data})
			if err := os.WriteFile(copyfile, txBuffer.Bytes(), 0644); err != nil {
				log.Fatal(err)
			}
			txBuffer.Reset()
			log.Println(receivedMessage)
		}
	}
}
