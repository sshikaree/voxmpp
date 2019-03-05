# voxmpp
Voice over XMPP app using base64 encoding.
## Usage
0. Install PortAudio dev files (on Ubuntu Linux: `$ sudo apt install portaudio19-dev`)
1. Install Opus dev files (on Ubuntu Linux: `$ sudo apt install libopus-dev libopusfile-dev`)
2. `$ go get -u github.com/sshikaree/voxmpp`
3. `$ go build`
4. `$ ./voxmpp -jid="user@example.com" -password="pass" -debug=false`

![Alt text](/screenshot/voxmpp.png?raw=true)

#### TODO
    ~~Call handshake~~
    ~~Audio compression~~