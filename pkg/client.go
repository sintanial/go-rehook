package rehook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	logger *zap.Logger

	reconnectTimeout time.Duration

	rules map[string]string
}

func NewClient(logger *zap.Logger, reconnectTimeout time.Duration, rules map[string]string) *Client {
	return &Client{
		logger:           logger,
		reconnectTimeout: reconnectTimeout,
		rules:            rules}
}

func (self *Client) Tunnel(addr string) error {
	for {
		if err := self.connect(addr); err != nil {
			self.logger.Info("wait to reconnect...", zap.String("addr", addr), zap.Duration("timeout", self.reconnectTimeout))
			time.Sleep(self.reconnectTimeout)
			self.logger.Info("try reconnect", zap.String("addr", addr))
		}
	}
}

func (self *Client) connect(addr string) error {
	self.logger.Debug("try connect to server", zap.String("addr", addr))
	conn, _, err := websocket.DefaultDialer.Dial(addr+"/_rehook/client", nil)
	if err != nil {
		self.logger.Error("failed connect to server", zap.String("addr", addr), zap.Error(err))
		return err
	}

	self.logger.Info("success connected to server", zap.String("addr", addr))
	defer conn.Close()

	var keys []string
	for key := range self.rules {
		keys = append(keys, key)
	}

	self.logger.Debug("try send register message", zap.String("addr", addr), zap.Any("keys", keys))
	if err := conn.WriteJSON(RegisterMessage{
		Keys: keys,
	}); err != nil {
		self.logger.Error("failed write register message to server", zap.String("addr", addr), zap.Any("keys", keys), zap.Error(err))
		return err
	}

	self.logger.Debug("client successfully registered", zap.String("addr", addr))
	for {
		var reqmsg RetransmitMessage
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				self.logger.Error("failed read retransmit message from server", zap.String("addr", addr), zap.Error(err))
			}
			return err
		}

		if err := json.Unmarshal(msgData, &reqmsg); err != nil {
			self.logger.Error("failed decode retransmit message from server", zap.String("addr", addr), zap.Error(err))
			return err
		}

		self.logger.Debug("incoming retransmit message, try decode body to http.Request", zap.String("addr", addr), zap.Any("msg", reqmsg))

		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(reqmsg.Body)))
		if err != nil {
			self.logger.Error("failed decode retransmit message body to http.Request", zap.String("addr", addr), zap.Any("msg", reqmsg), zap.Error(err))
			return err
		}

		receiver := self.rules[reqmsg.Key]

		u, err := url.Parse(receiver)
		if err != nil {
			self.logger.Error("failed parse receiver url", zap.String("addr", addr), zap.String("receiver", receiver), zap.Any("msg", reqmsg), zap.Error(err))
			return err
		}

		req.Host = u.Host

		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.URL.Path = u.Path
		req.RequestURI = ""

		self.logger.Debug("send retransmit message to receiver", zap.String("addr", addr), zap.Any("msg", reqmsg))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			self.logger.Error("failed retransmit request to receiver", zap.String("addr", addr), zap.Any("receiver", receiver), zap.Error(err))
			return err
		}

		buf := bytes.NewBuffer(nil)
		if err := resp.Write(buf); err != nil {
			self.logger.Debug("failed encode response", zap.String("addr", addr), zap.Any("msg", reqmsg))
			return err
		}

		resmsg := RetransmitMessage{
			ID:   reqmsg.ID,
			Key:  reqmsg.Key,
			Body: buf.Bytes(),
		}
		if err := conn.WriteJSON(&resmsg); err != nil {
			self.logger.Debug("failed send retransmit response message to server", zap.String("addr", addr), zap.Any("msg", reqmsg))
			return err
		}
	}
}
