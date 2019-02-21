# voxmpp
Voice over XMPP app using base64 encoding.
## Usage
0. Install PortAudio dev files (on Ubuntu Linux: `$ sudo apt install portaudio19-dev`)
1. `$ go get -u github.com/sshikaree/voxmpp`
2. `$ go build`
3. `$ ./voxmpp -jid="user@example.com" -password="pass" -debug=false`

####TODO
    -- Call handshake
    -- Audio compression