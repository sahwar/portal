package receiver

import (
	"fmt"
	"syscall"

	"github.com/gorilla/websocket"
	"www.github.com/ZinoKader/portal/models/protocol"
)

func (s *Sender) Transfer(wsConn *websocket.Conn) error {

	if s.ui != nil {
		defer close(s.ui)
	}

	s.state = WaitForFileRequest
	// messaging loop (with state variables)
	for {
		receivedMsg, err := readEncryptedMessage(wsConn, s.crypt)
		if err != nil {
			wsConn.Close()
			s.closeServer <- syscall.SIGTERM
			return fmt.Errorf("Shutting down portal due to websocket error: %s", err)
		}
		sendMsg := protocol.TransferMessage{}
		var wrongStateError *WrongStateError

		switch receivedMsg.Type {
		case protocol.ReceiverRequestPayload:
			if s.state != WaitForFileRequest {
				wrongStateError = NewWrongStateError(WaitForFileRequest, s.state)
				sendMsg = unsynchronizedErrorMsg
				break
			}

			err = s.streamPayload(wsConn)
			if err != nil {
				return err
			}
			sendMsg = protocol.TransferMessage{
				Type:    protocol.SenderPayloadSent,
				Payload: "Portal transfer completed",
			}
			s.state = WaitForFileAck
			s.updateUI()

		case protocol.ReceiverPayloadAck:
			if s.state != WaitForFileAck {
				wrongStateError = NewWrongStateError(WaitForFileAck, s.state)
				sendMsg = unsynchronizedErrorMsg
				break
			}
			s.state = WaitForCloseMessage
			s.updateUI()

			sendMsg = protocol.TransferMessage{
				Type:    protocol.SenderClosing,
				Payload: "Closing down the Portal, as requested",
			}
			s.state = WaitForCloseAck
			s.updateUI()

		case protocol.ReceiverClosingAck:
			if s.state != WaitForCloseAck {
				wrongStateError = NewWrongStateError(WaitForCloseAck, s.state)
			}
			wsConn.Close()
			s.closeServer <- syscall.SIGTERM
			// will be nil of nothing goes wrong.
			return wrongStateError

		case protocol.TransferError:
			s.updateUI()
			s.logger.Printf("Shutting down Portal due to Alien error")
			wsConn.Close()
			s.closeServer <- syscall.SIGTERM
			return nil
		}

		err = writeEncryptedMessage(wsConn, sendMsg, s.crypt)
		if err != nil {
			return nil
		}

		if wrongStateError != nil {
			wsConn.Close()
			s.closeServer <- syscall.SIGTERM
			return wrongStateError
		}
	}
}
