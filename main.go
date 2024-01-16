package main

import (
	"context"
	pb "copythrough/message/github.com/gregstarr/copythrough"
	"errors"
	"flag"
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
	pollInterval    time.Duration
)

func printMessage(msg *pb.Message) {
	if msg.Format == pb.Format_TEXT {
		log.Printf("%s: %s\n", msg.Origin, string(msg.Data))
	} else {
		log.Printf("%s: image\n", msg.Origin)
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

func readCopyFile() {
	data, err := os.ReadFile(copyfile)
	if err != nil {
		log.Fatal("Can't read copyfile:", err)
	}
	err = proto.Unmarshal(data, &receivedMessage)
	if err != nil {
		log.Fatal("Can't parse copyfile into protobuf:", err)
	}
	printMessage(&receivedMessage)
	if receivedMessage.Origin == host {
		return
	}
	clipboard.Write(convertFormat(receivedMessage.Format), receivedMessage.Data)
}

func writeCopyFile(data *[]byte, format pb.Format) {
	var (
		out []byte
		err error
	)
	msg := pb.Message{
		Origin: host,
		Format: format,
		Data:   *data,
	}
	if out, err = proto.Marshal(&msg); err != nil {
		log.Fatal("can't marshal protobuf:", err)
	}
	if err = os.WriteFile(copyfile, out, 0644); err != nil {
		log.Fatal("can't write copyfile:", err)
	}
	printMessage(&msg)
}

func watchFile() {
	var err error
	lastStatus := true
	_, err = os.Stat(copyfile)
	if errors.Is(err, os.ErrNotExist) {
		lastStatus = false
	} else if err != nil {
		log.Fatal("os stat error:", err)
	}
	for {
		_, err = os.Stat(copyfile)
		if err == nil {
			if lastStatus == false {
				readCopyFile()
			}
			lastStatus = true
		} else if errors.Is(err, os.ErrNotExist) {
			// file not exist
			lastStatus = false
		} else {
			log.Fatal("os stat error:", err)
		}
		time.Sleep(pollInterval)
	}
}

func init() {
	dir := flag.String("d", "D:", "directory to copy through (USB drive)")
	flag.DurationVar(&pollInterval, "poll", 250*time.Millisecond, "polling interval in ms")
	flag.Parse()

	copyfile = path.Join(*dir, ".COPYTHROUGH")

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
	var data []byte
	log.Printf("running copythrough on %s through %s...\n", host, copyfile)

	go watchFile()

	textChan := clipboard.Watch(context.Background(), clipboard.FmtText)
	imgChan := clipboard.Watch(context.Background(), clipboard.FmtImage)

	for {
		select {
		case data = <-textChan:
			writeCopyFile(&data, pb.Format_TEXT)
		case data = <-imgChan:
			writeCopyFile(&data, pb.Format_IMAGE)
		}
	}
}
