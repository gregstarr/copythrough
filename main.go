package main

import (
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

func readCopyFile() error {
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

func main() {
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

	go func() {
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
				if err = readCopyFile(); err != nil {
					log.Fatal(err)
				}
			}
			if err == nil {
				laststatus = true
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	textChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	imgChan := clipboard.Watch(context.Background(), clipboard.FmtImage)
	for {
		select {
		case data := <-textChan:
			err = writeCopyFile(&data, pb.Format_TEXT)
		case data := <-imgChan:
			err = writeCopyFile(&data, pb.Format_IMAGE)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
