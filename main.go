package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/gordonklaus/portaudio"

	audio "github.com/sshikaree/b64audio"
	xmpp "github.com/sshikaree/go-xmpp2"
)

func main() {
	log.SetFlags(log.Lshortfile)

	runtime.LockOSThread()

	// var remote_jid string = ""

	jid := flag.String("jid", "", "jid")
	password := flag.String("password", "", "password")
	notls := flag.Bool("notls", true, "No TLS")
	debug := flag.Bool("debug", false, "Debug mode")

	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "" {
			fmt.Println(f.Name, "is empty!")
			os.Exit(1)
		}
	})

	client := NewApp(jid, password, *notls, *debug)
	// client.ConnectXMPPAndRetry()
	err := client.ConnectToXMPPServer()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Init PortAudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer portaudio.Terminate()

	// Periodicaly send ping to server
	go func() {
		for {
			<-time.After(PING_INTERVAL)
			if client.Client != nil {
				if err := client.PingC2S("", ""); err != nil {
					log.Println(err)
				}
			} else {
				// reconnect
				log.Println("Connection to xmpp server lost. Reconnecting...")
				client.ConnectXMPPAndRetry()
			}
		}
	}()

	// XMPP reader
	go func() {
		for {
			chat, err := client.Recv()
			if err != nil {
				log.Println(err)
				continue
			}
			switch v := chat.(type) {
			case xmpp.Message:
				// TODO:
				// - run ParseXMPPMessage in a separate goroutine?
				go client.ParseXMPPMessage(&v)
			case xmpp.Presence:
				continue
			case xmpp.IQ:
				continue
			}
		}
	}()

	// Chunks sender
	go func() {
		var (
			chunk []byte
			err   error
		)
		for {
			// log.Println("Sending...")
			chunk = <-client.OutgoingBuffer
			if client.RemoteJID.Get() == "" {
				continue
			}
			if err = client.SendChunk(chunk, client.RemoteJID.Get()); err != nil {
				log.Println("Error sending chunk:", err)
			}
		}

	}()

	// Player
	go func() {
		if err := audio.ContinuousPlayOpus(client.IncomingBuffer); err != nil {
			log.Fatal(err)
		}
	}()

	// Audio Recorder
	go func() {
		// log.Println("Recording...")
		if err := audio.ContinuousRecordOpus(client.OutgoingBuffer); err != nil {
			log.Fatal(err)
		}

	}()

	// scanner := bufio.NewScanner(os.Stdin)
	// fmt.Print("~> ")

	// wait for exit command
	// <-client.ExitCh

	// for client.scanner.Scan() {
	// 	client.ParseCommandLine(client.scanner.Text())
	// 	fmt.Print("~> ")
	// }

	// Run User Interface
	if err := client.ui.Run(); err != nil {
		log.Fatal(err)
	}
}
