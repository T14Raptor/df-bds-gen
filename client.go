package gen

import (
	"errors"
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"sync"
	"time"
)

type Client struct {
	started atomic.Bool

	conn *minecraft.Conn

	id uint64

	log *logrus.Logger

	cacheMu    sync.Mutex
	chunkCache map[world.ChunkPos]ChunkData

	expectancyMu      sync.Mutex
	chunkExpectancies map[world.ChunkPos]chan ChunkData
}

func NewClient(log *logrus.Logger) *Client {
	return &Client{
		log:               log,
		chunkCache:        make(map[world.ChunkPos]ChunkData),
		chunkExpectancies: make(map[world.ChunkPos]chan ChunkData),
	}
}

func (c *Client) StartClient(address string, chunkRadius int) error {
	if !c.started.CAS(false, true) {
		return errors.New("client already started")
	}

	c.log.Debugln("Starting client...")
	conn, err := minecraft.Dial("raknet", address)
	if err != nil {
		return fmt.Errorf("error starting client: %s", err.Error())
	}

	if err := conn.DoSpawn(); err != nil {
		return fmt.Errorf("error starting client: %s", err.Error())
	}

	c.conn = conn
	c.id = conn.GameData().EntityRuntimeID

	c.log.Debugln("Client spawned in.")

	runtimeIdToState = make([]Block, len(c.conn.GameData().Blocks))
	for runtimeID, b := range c.conn.GameData().Blocks {
		m := b.(map[string]interface{})["block"].(map[string]interface{})
		runtimeIdToState[runtimeID] = Block{Name: m["name"].(string), States: m["states"].(map[string]interface{})}
	}

	_ = c.conn.WritePacket(&packet.RequestChunkRadius{ChunkRadius: int32(chunkRadius)})

	go c.ReadPackets()
	go c.chunkCacheJanitor()

	return nil
}

func (c *Client) ReadPackets() {
	defer c.conn.Close()
	for {
		pk, err := c.conn.ReadPacket()
		if err != nil {
			break
		}

		c.handlePacket(pk)
	}
}

func (c *Client) handlePacket(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.LevelChunk:
		chunkData := ChunkData{
			Payload:       pk.RawPayload,
			SubChunkCount: pk.SubChunkCount,
		}

		pos := world.ChunkPos{pk.ChunkX, pk.ChunkZ}

		c.expectancyMu.Lock()
		if ch, ok := c.chunkExpectancies[pos]; ok {
			delete(c.chunkExpectancies, pos)
			c.expectancyMu.Unlock()

			ch <- chunkData
			return
		}
		c.expectancyMu.Unlock()

		c.cacheMu.Lock()
		c.chunkCache[pos] = chunkData
		c.cacheMu.Unlock()
	}
}

func (c *Client) chunkCacheJanitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	for range t.C {
		c.cacheMu.Lock()
		c.chunkCache = make(map[world.ChunkPos]ChunkData)
		c.cacheMu.Unlock()
	}
}

func (c *Client) Chunk(pos world.ChunkPos) (ChunkData, bool) {
	c.cacheMu.Lock()
	ch, ok := c.chunkCache[pos]
	if ok {
		delete(c.chunkCache, pos)
	}
	c.cacheMu.Unlock()

	return ch, ok
}

func (c *Client) ExpectChunk(pos world.ChunkPos, ch chan ChunkData) {
	c.expectancyMu.Lock()
	c.chunkExpectancies[pos] = ch
	c.expectancyMu.Unlock()
}

func (c *Client) StopExpecting(pos world.ChunkPos) {
	c.expectancyMu.Lock()
	delete(c.chunkExpectancies, pos)
	c.expectancyMu.Unlock()
}

func (c *Client) Conn() *minecraft.Conn {
	return c.conn
}
