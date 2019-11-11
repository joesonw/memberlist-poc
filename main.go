package main

import (
	"encoding/json"
	"flag"
	"github.com/hashicorp/memberlist"
	"io/ioutil"
	"net/http"
	"strings"
)

type BroadcastBody struct {
	Data   *Data  `json:"data,omitempty"`
	Node   string `json:"node,omitempty"`
	Action string `json:"action,omitempty"`
}

type Broadcast struct {
	message []byte
}

func newBroadcast(data *Data, node, action string) *Broadcast {
	body := BroadcastBody{
		Data:   data,
		Node:   node,
		Action: action,
	}
	message, err := json.Marshal(&body)
	die(err)

	return &Broadcast{
		message: message,
	}
}

func (m Broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (m Broadcast) Finished() {

}

func (m Broadcast) Message() []byte {
	return m.message
}

type Data struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Entry struct {
	Data       *Data  `json:"data,omitempty"`
	SourceNode string `json:"SourceNode,omitempty"`
}

type Server struct {
	list       *memberlist.Memberlist
	broadcasts *memberlist.TransmitLimitedQueue
	cache      map[int64]*Entry
}

func (s *Server) NotifyJoin(node *memberlist.Node) {
}

func (s *Server) NotifyLeave(node *memberlist.Node) {
}

func (s *Server) NotifyUpdate(node *memberlist.Node) {
	for id, entry := range s.cache {
		if entry.SourceNode == node.Name {
			delete(s.cache, id)
		}
	}
}

func (s *Server) NodeMeta(limit int) []byte {
	return nil
}

func (s *Server) NotifyMsg(msg []byte) {
	message := BroadcastBody{}
	die(json.Unmarshal(msg, &message))
	if message.Action == "create" {
		s.cache[message.Data.ID] = &Entry{
			Data:       message.Data,
			SourceNode: message.Node,
		}
	}
}

func (s *Server) GetBroadcasts(overhead, limit int) [][]byte {
	return s.broadcasts.GetBroadcasts(overhead, limit)
}

func (s *Server) LocalState(join bool) []byte {
	var list []*Entry
	for _, entry := range s.cache {
		list = append(list, entry)
	}
	state, err := json.Marshal(&list)
	die(err)
	return state
}

func (s *Server) MergeRemoteState(body []byte, join bool) {
	var list []*Entry
	die(json.Unmarshal(body, &list))
	if join {
		for _, entry := range list {
			s.cache[entry.Data.ID] = entry
		}
	}
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var (
		err  error
		body []byte
	)

	defer func() {
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
		} else {
			res.WriteHeader(200)
			res.Write(body)
		}
	}()

	if req.Method == http.MethodGet {
		var list []*Data
		for _, entry := range s.cache {
			list = append(list, entry.Data)
		}
		body, err = json.Marshal(&list)
		return
	}

	if req.Method == http.MethodPost {
		body, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return
		}

		data := &Data{}
		err = json.Unmarshal(body, data)
		if err != nil {
			return
		}
		if s.cache[data.ID] != nil {
			body = []byte("exists")
			return
		}
		s.cache[data.ID] = &Entry{
			Data:       data,
			SourceNode: s.list.LocalNode().Name,
		}
		s.broadcasts.QueueBroadcast(newBroadcast(data, s.list.LocalNode().Name, "create"))
		body = []byte("ok")
		return
	}
}

func main() {
	portP := flag.Int("port", 0, "port")
	listenP := flag.String("listen", ":0", "listen")
	nameP := flag.String("name", "", "name")
	peersP := flag.String("peers", "", "peers")
	flag.Parse()

	server := &Server{
		cache: make(map[int64]*Entry),
	}

	config := memberlist.DefaultLocalConfig()
	config.BindPort = *portP
	config.AdvertisePort = *portP
	config.Name = *nameP
	config.Events = server
	config.Delegate = server

	list, err := memberlist.Create(config)
	die(err)
	server.list = list
	server.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       list.NumMembers,
		RetransmitMult: 3,
	}

	var peers []string
	if len(*peersP) > 0 {
		peers = strings.Split(*peersP, ",")
	}

	if len(peers) > 0 {
		_, err = list.Join(peers)
		die(err)
	}

	die(http.ListenAndServe(*listenP, server))
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}
