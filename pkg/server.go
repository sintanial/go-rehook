package rehook

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-uuid"
	"go.uber.org/zap"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	logger *zap.Logger

	retransmitTimeout time.Duration

	mu       sync.Mutex
	clients  map[string]*websocket.Conn
	respchan map[string]map[string]chan *responseMessage
}

func NewServer(logger *zap.Logger, retransmitTimeout time.Duration) *Server {
	return &Server{
		logger: logger,

		retransmitTimeout: retransmitTimeout,

		clients:  make(map[string]*websocket.Conn),
		respchan: make(map[string]map[string]chan *responseMessage),
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (self *Server) Serve(l net.Listener, handler *http.ServeMux) error {
	self.Register(handler)
	return http.Serve(l, handler)
}

func (self *Server) ListenAndServe(addr string, handler *http.ServeMux) error {
	self.Register(handler)
	return http.ListenAndServe(addr, handler)
}

func (self *Server) ListenAndServeTLS(addr string, certFile string, keyFile string, handler *http.ServeMux) error {
	self.Register(handler)
	return http.ListenAndServeTLS(addr, certFile, keyFile, handler)
}

func (self *Server) Register(handler *http.ServeMux) {
	handler.HandleFunc("/_rehook/client", self.ClientHandler)
	handler.HandleFunc("/", self.RetransmitHandler)
}

func (self *Server) ClientHandler(w http.ResponseWriter, r *http.Request) {
	dumpreq := self.getDumpedRequest(w, r, true)
	if dumpreq == nil {
		return
	}

	self.logger.Debug("incoming client connect", zap.Any("req", dumpreq))

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		self.logger.Error("failed upgrade client request to websocket", zap.Any("req", dumpreq), zap.Error(err))
		return
	}

	self.logger.Debug("try read register message", zap.Any("req", dumpreq))
	var regmsg RegisterMessage
	if err := conn.ReadJSON(&regmsg); err != nil {
		self.logger.Error("failed read register message", zap.Any("req", dumpreq), zap.Error(err))
		return
	}

	self.logger.Debug("register client in system", zap.Any("req", dumpreq), zap.Error(err))
	for _, key := range regmsg.Keys {
		self.setClient(key, conn)
	}
	defer func() {
		self.logger.Debug("deregister client in system", zap.Any("req", dumpreq), zap.Error(err))

		for _, key := range regmsg.Keys {
			self.rmClient(key, conn)
		}
	}()

	self.logger.Debug("client successfully connected", zap.Any("req", dumpreq))

	for {
		var data []byte
		_, data, err = conn.ReadMessage()
		if err != nil {
			self.logger.Error("failed read server message", zap.Any("req", dumpreq), zap.Error(err))
			break
		}

		var msg RetransmitMessage
		if err = json.Unmarshal(data, &msg); err != nil {
			self.logger.Error("failed decode client response retransmit message", zap.Any("req", dumpreq), zap.String("data", string(data)), zap.Error(err))
			break
		}

		ch := self.getResponseChan(conn.RemoteAddr().String(), msg.ID)
		if ch == nil {
			self.logger.Panic("response channil is <nil>", zap.Any("req", dumpreq), zap.String("data", string(data)), zap.Error(err))
			break
		}

		ch <- &responseMessage{Message: &msg}
	}

	self.mu.Lock()
	defer self.mu.Unlock()

	ids, ok := self.respchan[conn.RemoteAddr().String()]
	if ok {
		for _, ch := range ids {
			ch <- &responseMessage{Error: err}
		}
	}

	self.logger.Debug("client disconnected", zap.Any("req", dumpreq))
}

func (self *Server) setClient(key string, conn *websocket.Conn) {
	self.mu.Lock()
	defer self.mu.Unlock()

	if prev, ok := self.clients[key]; ok {
		prev.Close()
	}

	self.clients[key] = conn
}

func (self *Server) rmClient(key string, prev *websocket.Conn) {
	self.mu.Lock()
	defer self.mu.Unlock()

	prev, ok := self.clients[key]
	if !ok {
		return
	}

	if prev.RemoteAddr().String() != prev.RemoteAddr().String() {
		return
	}

	delete(self.clients, key)
}

func (self *Server) getConn(key string) *websocket.Conn {
	self.mu.Lock()
	defer self.mu.Unlock()

	return self.clients[key]
}

func (self *Server) RetransmitHandler(w http.ResponseWriter, r *http.Request) {
	dumpreq := self.getDumpedRequest(w, r, true)
	if dumpreq == nil {
		return
	}

	self.logger.Debug("incoming retransmit request", zap.Any("req", dumpreq))

	key := r.URL.Path

	conn := self.getConn(key)
	if conn == nil {
		w.WriteHeader(http.StatusBadGateway)
		self.logger.Error("suitable connect not found", zap.Any("req", dumpreq), zap.Any("key", key))
		return
	}

	buf := bytes.NewBuffer(nil)
	if err := r.Write(buf); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		self.logger.Error("failed encode request", zap.Any("req", dumpreq), zap.Error(err))
		return
	}

	id, _ := uuid.GenerateUUID()
	reqmsg := RetransmitMessage{
		Key:  key,
		ID:   id,
		Body: buf.Bytes(),
	}

	self.logger.Debug("try send retransmit message to client", zap.Any("req", dumpreq))
	if err := conn.WriteJSON(&reqmsg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		self.logger.Error("failed send retransmit message to client", zap.Any("req", dumpreq), zap.Any("reqmsg", reqmsg), zap.Error(err))
		return
	}

	ch := self.createResponseChan(conn.RemoteAddr().String(), id)
	if self.retransmitTimeout > 0 {
		ts := time.NewTimer(self.retransmitTimeout)
		defer ts.Stop()

		self.logger.Debug("waiting response from client ...", zap.Any("req", dumpreq), zap.Any("reqmsg", reqmsg))
		select {
		case msg := <-ch:
			self.delResponseChan(conn.RemoteAddr().String(), id)
			if err := self.handleRetransmitResponse(w, msg.Message); err != nil {
				self.logger.Error("failed retransmit response", zap.Any("req", dumpreq), zap.Any("reqmsg", reqmsg), zap.Any("resmsg", msg), zap.Error(err))
			}
		case <-ts.C:
			self.delResponseChan(conn.RemoteAddr().String(), id)

			w.WriteHeader(http.StatusGatewayTimeout)
			self.logger.Error("client retransmit response timeout", zap.Any("req", dumpreq), zap.Any("reqmsg", reqmsg))
			return
		}
	} else {
		msg := <-ch
		self.delResponseChan(conn.RemoteAddr().String(), id)
		if err := self.handleRetransmitResponse(w, msg.Message); err != nil {
			self.logger.Error("failed retransmit response", zap.Any("req", dumpreq), zap.Any("reqmsg", reqmsg), zap.Any("resmsg", msg), zap.Error(err))
		}
	}
}

func (self *Server) handleRetransmitResponse(w http.ResponseWriter, msg *RetransmitMessage) error {
	h, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return errors.New("failed hijack request")
	}

	conn, _, err := h.Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	conn.Write(msg.Body)
	conn.Close()

	return nil
}

func (self *Server) createResponseChan(client string, id string) chan *responseMessage {
	ch := make(chan *responseMessage)

	self.mu.Lock()
	defer self.mu.Unlock()

	ids, ok := self.respchan[client]
	if !ok {
		ids = make(map[string]chan *responseMessage)
		self.respchan[client] = ids
	}

	ids[id] = ch

	return ch
}

func (self *Server) getResponseChan(client string, id string) chan *responseMessage {
	self.mu.Lock()
	defer self.mu.Unlock()

	ids, ok := self.respchan[client]
	if !ok {
		return nil
	}

	return ids[id]
}

func (self *Server) delResponseChan(client string, id string) {
	self.mu.Lock()
	defer self.mu.Unlock()

	ids, ok := self.respchan[client]
	if !ok {
		return
	}

	delete(ids, id)
}
