package main

import (
	"context"
	pb "copythrough/message/github.com/gregstarr/copythrough"
	"errors"
	"github.com/fsnotify/fsnotify"
	"golang.design/x/clipboard"
	"google.golang.org/protobuf/proto"
	"log"
	"os"
	"path"
	"time"
)

func convertFormat(format pb.Format) clipboard.Format {
	switch format {
	case pb.Format_TEXT:
		return clipboard.FmtText
	case pb.Format_IMAGE:
		return clipboard.FmtImage
	default:
		return clipboard.FmtText
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

	var receivedMessage pb.Message

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
					err = proto.Unmarshal(data, &receivedMessage)
					if err != nil {
						log.Fatal(err)
					}
					log.Println(receivedMessage.String())
					if receivedMessage.Origin == host {
						continue
					}
					clipboard.Write(convertFormat(receivedMessage.Format), receivedMessage.Data)
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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
	for {
		select {
		case data := <-textChan:
			msg := pb.Message{
				Origin: host,
				Format: pb.Format_TEXT,
				Data:   data,
			}
			out, err := proto.Marshal(&msg)
			if err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(copyfile, out, 0644); err != nil {
				log.Fatal(err)
			}
			log.Println(msg.String())
		case data := <-imgChan:
			msg := pb.Message{
				Origin: host,
				Format: pb.Format_IMAGE,
				Data:   data,
			}
			if err != nil {
				log.Fatal(err)
			}
			out, err := proto.Marshal(&msg)
			if err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(copyfile, out, 0644); err != nil {
				log.Fatal(err)
			}
			log.Println(msg.String())
		}
	}
}
