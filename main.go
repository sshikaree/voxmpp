package main

import (
	"log"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"

	audio "github.com/sshikaree/b64audio"
	xmpp "github.com/sshikaree/go-xmpp2"
)

func main() {
	log.SetFlags(log.Lshortfile)

	if len(os.Args) < 2 {
		log.Printf("Usage: %s JID (e.g.: %s user@example.com)\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}

	opponentJID := os.Args[1]

	client := NewApp()
	client.ConnectXMPPAndRetry()
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
				client.ParseXMPPMessage(&v)
			case xmpp.Presence:
				continue
			case xmpp.IQ:
				continue
			}
		}
	}()

	// Audio Recorder
	go func() {
		// log.Println("Recording...")
		if err := audio.ContinuousRecord(client.OutgoingBuffer); err != nil {
			log.Fatal(err)
		}

	}()

	// Sender
	go func() {
		var (
			chunk []byte
			err   error
		)
		for {
			// log.Println("Sending...")
			chunk = <-client.OutgoingBuffer
			if err = client.SendChunk(chunk, opponentJID); err != nil {
				log.Println(err)
			}
		}

	}()

	// Player
	// var chunk []byte
	// for {
	// 	chunk = <-client.IncomingBuffer
	// 	audio.PlayChunk(chunk)
	// }
	if err := audio.ContinuousPlay(client.IncomingBuffer); err != nil {
		log.Fatal(err)
	}

}
