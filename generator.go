package gen

import (
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"time"
)

type Generator struct {
	c *Client
}

func NewGenerator(c *Client) Generator {
	return Generator{c: c}
}

func (g Generator) GenerateChunk(pos world.ChunkPos, ch *chunk.Chunk) {
	c, ok := g.c.Chunk(pos)
	if ok {
		decodedChunk := decodeNetworkChunk(c)
		//noinspection GoVetCopyLock
		*ch = *decodedChunk
		return
	}

	teleportPos := mgl32.Vec3{float32(pos.X()<<4) + 10.0, 255.0, float32(pos.Z()<<4) + 10.0}
	_ = g.c.conn.WritePacket(&packet.MovePlayer{
		EntityRuntimeID: g.c.id,
		Position:        teleportPos,
	})

	cha := make(chan ChunkData, 1)
	defer close(cha)

	g.c.ExpectChunk(pos, cha)
	t := time.After(1 * time.Second)
	for {
		select {
		case data := <-cha:
			decodedChunk := decodeNetworkChunk(data)
			//noinspection GoVetCopyLock
			*ch = *decodedChunk
			return
		case <-t:
			g.c.StopExpecting(pos)
			g.c.log.Warnln("Failed to get chunk", pos, "from BDS.")
			return
		default:
			_ = g.c.conn.WritePacket(&packet.MovePlayer{
				EntityRuntimeID: g.c.id,
				Position:        teleportPos,
			})
		}
	}
}

func decodeNetworkChunk(data ChunkData) *chunk.Chunk {
	c, _ := chunk.NetworkDecode(data.Payload, int(data.SubChunkCount))

	for _, s := range c.Sub() {
		if s == nil {
			continue
		}

		for _, l := range s.Layers() {
			l.Palette().Replace(func(r uint32) uint32 {
				block := runtimeIdToState[r]

				runtimeId, ok := chunk.StateToRuntimeID(block.Name, block.States)
				if ok {
					return runtimeId
				}

				// return old runtime id because fukkit
				return r
			})
		}
	}

	return c
}
