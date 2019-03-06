package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	xmpp "github.com/sshikaree/go-xmpp2"
)

func main() {
	log.SetFlags(log.Lshortfile)

	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

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
	// log.SetOutput(client.ui.textView)

	// client.ConnectXMPPAndRetry()
	err := client.ConnectToXMPPServer()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Init PortAudio
	// if err := portaudio.Initialize(); err != nil {
	// 	log.Fatal(err)
	// }
	// defer portaudio.Terminate()

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
				// TODO:
				// - reconnect ??
				log.Println(err)

				client.ConnectXMPPAndRetry()
				continue
			}
			switch v := chat.(type) {
			case xmpp.Message:
				client.ParseXMPPMessage(&v)
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
			chunk = <-client.OutgoingBuffer
			if client.RemoteJID.Get() == "" {
				continue
			}
			if err = client.SendChunk(chunk, client.RemoteJID.Get()); err != nil {
				log.Println("Error sending chunk:", err)
			}
		}

	}()

	// Init audio
	audio, err := InitAudio(client.IncomingBuffer, client.OutgoingBuffer)
	if err != nil {
		log.Fatal(err)
	}
	defer audio.Close()

	// Run User Interface
	if err := client.ui.Run(); err != nil {
		log.Fatal(err)
	}
}
