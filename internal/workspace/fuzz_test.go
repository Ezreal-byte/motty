package workspace

import "testing"

func FuzzLayoutOperations(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{1, 255, 0, 8, 2, 11, 3, 19, 4, 27})
	f.Fuzz(func(t *testing.T, data []byte) {
		layout, err := New(1)
		if err != nil {
			t.Fatal(err)
		}
		for index := 0; index+3 < len(data); index += 4 {
			session := SessionID(data[index+1]%32 + 1)
			target := SessionID(data[index+2]%32 + 1)
			ratio := float32(data[index+3]%99+1) / 100
			axis := Horizontal
			if data[index]&1 != 0 {
				axis = Vertical
			}
			placement := Before
			if data[index]&2 != 0 {
				placement = After
			}
			switch data[index] % 4 {
			case 0:
				_ = layout.Split(target, session, axis, ratio, placement)
			case 1:
				if layout.Len() > 1 {
					_ = layout.Remove(session)
				}
			case 2:
				_ = layout.Move(session, target, axis, ratio, placement)
			case 3:
				_ = layout.SetRatio(layout.Root(), ratio)
			}
			if err := layout.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		dto := layout.DTO()
		restored, err := Restore(dto)
		if err != nil {
			t.Fatal(err)
		}
		assertDTOEqual(t, restored.DTO(), dto)
	})
}

func FuzzRestoreRejectsMalformedDTO(f *testing.F) {
	f.Add([]byte{1, 1, 1, 0, 0, 0, 0, 0})
	f.Add([]byte{2, 2, 1, 50, 2, 3, 1, 9, 1, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		count := len(data) / 6
		if count > 64 {
			count = 64
		}
		dto := DTO{Version: DTOVersion, Root: 1, Nodes: make([]NodeDTO, count)}
		for index := range dto.Nodes {
			offset := index * 6
			dto.Nodes[index] = NodeDTO{
				ID: NodeID(index + 1), Kind: Kind(data[offset] % 3), Session: SessionID(data[offset+1]),
				Axis: Axis(data[offset+2] % 4), Ratio: float32(data[offset+3]) / 255,
				First: NodeID(data[offset+4]), Second: NodeID(data[offset+5]),
			}
		}
		if count == 0 {
			dto.Root = 0
		}
		layout, err := Restore(dto)
		if err != nil {
			return
		}
		if err := layout.Validate(); err != nil {
			t.Fatal(err)
		}
		if err := layout.DTO().Validate(); err != nil {
			t.Fatal(err)
		}
	})
}
