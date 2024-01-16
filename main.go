package main

import (
	"bytes"
	"context"
	pb "copythrough/message/github.com/gregstarr/copythrough"
	"errors"
	"fmt"
	"golang.design/x/clipboard"
	"google.golang.org/protobuf/proto"
	"log"
	"os"
	"path"
	"time"
)

var (
	copyfile        string
	receivedMessage pb.Message
	host            string
)

func printMessage(msg *pb.Message) {
	if msg.Format == pb.Format_TEXT {
		fmt.Printf("%s: %s\n", msg.Origin, string(msg.Data))
	} else {
		fmt.Printf("%s: image\n", msg.Origin)
	}
}

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

func readCopyFile(selfChan *chan []byte) error {
	data, err := os.ReadFile(copyfile)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(data, &receivedMessage)
	if err != nil {
		return err
	}
	printMessage(&receivedMessage)
	if receivedMessage.Origin == host {
		return nil
	}
	*selfChan <- receivedMessage.Data
	clipboard.Write(convertFormat(receivedMessage.Format), receivedMessage.Data)
	return nil
}

func writeCopyFile(data *[]byte, format pb.Format) error {
	var (
		out []byte
		err error
	)
	msg := pb.Message{
		Origin: host,
		Format: format,
		Data:   *data,
	}
	out, err = proto.Marshal(&msg)
	if err != nil {
		return err
	}
	if err = os.WriteFile(copyfile, out, 0644); err != nil {
		return err
	}
	printMessage(&msg)
	return nil
}

func watchFile(selfChan *chan []byte) {
	var err error
	laststatus := true
	_, err = os.Stat(copyfile)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		laststatus = false
	}
	for {
		_, err = os.Stat(copyfile)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			// file not exist
			laststatus = false
		}
		if err == nil && laststatus == false {
			if err = readCopyFile(selfChan); err != nil {
				log.Fatal(err)
			}
		}
		if err == nil {
			laststatus = true
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func init() {
	dir := os.Args[1]
	copyfile = path.Join(dir, ".COPYTHROUGH")

	var err error
	if err = clipboard.Init(); err != nil {
		log.Fatal(err)
	}

	host, err = os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var (
		err  error
		data []byte
	)

	selfChan := make(chan []byte)

	go watchFile(&selfChan)

	textChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	imgChan := clipboard.Watch(context.Background(), clipboard.FmtImage)

	for {
		select {
		case data = <-selfChan:
			err = writeCopyFile(&data, pb.Format_TEXT)
		case d := <-textChan:
			if bytes.Equal(d, data) {
				continue
			}
			err = writeCopyFile(&d, pb.Format_TEXT)
		case d := <-imgChan:
			if bytes.Equal(d, data) {
				continue
			}
			err = writeCopyFile(&d, pb.Format_IMAGE)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
