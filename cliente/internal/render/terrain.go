package render

import (
	"FortressVision/cliente/internal/mesher"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"
	"math"
	"sync"

	"kaijuengine.com/engine"
	"kaijuengine.com/matrix"
	"kaijuengine.com/registry/shader_data_registry"
	"kaijuengine.com/rendering"
)

type unitDrawing struct {
	transform  *matrix.Transform
	shaderData rendering.DrawInstance
}

type unitPosition struct {
	x, y, z matrix.Float
}

type unitEquipmentDrawing struct {
	drawing *unitDrawing
	offset  unitPosition
	scale   unitPosition
}

// tileSurfaceOffset Ã© a altura local onde termina o voxel do terreno. Itens,
// manchas e marcaÃ§Ãµes do DF ficam apoiados nessa superfÃ­cie; usar apenas a
// coordenada-base do tile os deixa enterrados ou visualmente desalinhados.
const tileSurfaceOffset matrix.Float = 1.0

// tileOverlayThickness/epsilon mantêm overlays coplanares com o piso sem
// deixá-los atravessar o terreno nem parecerem placas suspensas.
const tileOverlayThickness matrix.Float = 0.012
const tileOverlayEpsilon matrix.Float = 0.001

func tileSurfaceY(tileBaseY, localOffset matrix.Float) matrix.Float {
	return tileBaseY + tileSurfaceOffset + localOffset
}

func (p unitPosition) vec() matrix.Vec3 {
	return matrix.NewVec3(p.x, p.y, p.z)
}

func lerpUnitPosition(current, target unitPosition, alpha matrix.Float) unitPosition {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return unitPosition{
		x: current.x + (target.x-current.x)*alpha,
		y: current.y + (target.y-current.y)*alpha,
		z: current.z + (target.z-current.z)*alpha,
	}
}

type chunkDrawing struct {
	mesh       *rendering.Mesh
	meshKey    string
	capacity   int
	transform  *matrix.Transform
	shaderData rendering.DrawInstance
}

type chunkEntityUpdate struct {
	origin   util.DFCoord
	snapshot mapdata.ChunkSnapshot
}

// TerrainRenderer gerencia a exibição do terreno 3D na engine Kaiju.
type TerrainRenderer struct {
	host              *engine.Host
	mu                sync.Mutex
	queueMu           sync.Mutex
	meshingMgr        *mesher.Manager
	chunkDrawings     map[string]map[int32]*chunkDrawing
	terrainMaterial   *rendering.Material
	liquidMaterial    *rendering.Material
	unitDrawings      map[int32]*unitDrawing
	unitTargets       map[int32]unitPosition
	unitPositions     map[int32]unitPosition
	unitEquipment     map[int32]map[string]*unitEquipmentDrawing
	unitAppearance    map[int32]map[string]*unitEquipmentDrawing
	unitMesh          *rendering.Mesh
	unitMaterial      *rendering.Material
	entityMeshes      map[string]*rendering.Mesh
	entityDrawings    map[string]*unitDrawing
	chunkEntityKeys   map[string]map[string]struct{}
	entityKeyRefs     map[string]map[string]struct{}
	pendingMeshes     map[string]*mesher.ChunkMesh
	pendingEntities   map[string]chunkEntityUpdate
	invalidated       map[string]struct{}
	chunkVersions     map[string]uint64
	meshFlushQueued   bool
	entityFlushQueued bool
}

// NewTerrainRenderer inicializa um novo renderizador de terreno desacoplado.
func NewTerrainRenderer(host *engine.Host, meshingMgr *mesher.Manager) *TerrainRenderer {
	tr := &TerrainRenderer{
		host:            host,
		meshingMgr:      meshingMgr,
		chunkDrawings:   make(map[string]map[int32]*chunkDrawing),
		unitDrawings:    make(map[int32]*unitDrawing),
		unitTargets:     make(map[int32]unitPosition),
		unitPositions:   make(map[int32]unitPosition),
		unitEquipment:   make(map[int32]map[string]*unitEquipmentDrawing),
		unitAppearance:  make(map[int32]map[string]*unitEquipmentDrawing),
		entityMeshes:    make(map[string]*rendering.Mesh),
		entityDrawings:  make(map[string]*unitDrawing),
		chunkEntityKeys: make(map[string]map[string]struct{}),
		entityKeyRefs:   make(map[string]map[string]struct{}),
		pendingMeshes:   make(map[string]*mesher.ChunkMesh),
		pendingEntities: make(map[string]chunkEntityUpdate),
		invalidated:     make(map[string]struct{}),
		chunkVersions:   make(map[string]uint64),
	}

	// Vincular evento de geração de malha.
	tr.meshingMgr.OnMeshGenerated = tr.onMeshGenerated

	return tr
}

// UpdateUnits recebe um snapshot do DFHack e agenda a atualização no thread da
// engine. As transformações existentes são reaproveitadas para não acumular
// desenhos nem deixar unidades antigas como fantasmas.
func (tr *TerrainRenderer) UpdateUnits(units []mapdata.UnitInstance) {
	snapshot := append([]mapdata.UnitInstance(nil), units...)
	tr.host.RunOnMainThread(func() {
		tr.applyUnits(snapshot)
	})
}

func (tr *TerrainRenderer) applyUnits(units []mapdata.UnitInstance) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.ensureEntityResources() {
		return
	}

	seen := make(map[int32]struct{}, len(units))
	for _, unit := range units {
		seen[unit.ID] = struct{}{}
		drawing, ok := tr.unitDrawings[unit.ID]
		if !ok {
			sd := shader_data_registry.Create("basic")
			if sd == nil {
				continue
			}
			if standard, ok := sd.(*shader_data_registry.ShaderDataStandard); ok {
				standard.Color = matrix.NewColor(0.75, 0.9, 1.0, 1.0)
			}
			transform := &matrix.Transform{}
			transform.SetupRawTransform()
			transform.SetScale(matrix.NewVec3(0.35, 0.7, 0.35))
			drawing = &unitDrawing{
				transform:  transform,
				shaderData: sd,
			}
			tr.unitDrawings[unit.ID] = drawing
			tr.host.Drawings.AddDrawing(rendering.Drawing{
				Mesh:       tr.unitMesh,
				Material:   tr.unitMaterial,
				ShaderData: sd,
				Transform:  transform,
				ViewCuller: &CameraViewCuller{Camera: tr.host.PrimaryCamera()},
			})
		}

		if unit.IsValid() {
			drawing.shaderData.Activate()
			if standard, ok := drawing.shaderData.(*shader_data_registry.ShaderDataStandard); ok {
				standard.Color = unitColor(unit)
			}
			scale := unitScale(unit)
			drawing.transform.SetScale(matrix.NewVec3(0.35*scale, 0.7*scale, 0.35*scale))
			pos := util.DFToWorldPos(unit.Pos)
			target := unitPosition{
				x: matrix.Float(pos.X) + 0.5 + matrix.Float(unit.SubPos.X),
				y: tileSurfaceY(matrix.Float(pos.Y), 0.35*scale+matrix.Float(unit.SubPos.Z)),
				z: matrix.Float(pos.Z) - 0.5 - matrix.Float(unit.SubPos.Y),
			}
			if _, exists := tr.unitPositions[unit.ID]; !exists {
				tr.unitPositions[unit.ID] = target
				drawing.transform.SetPosition(target.vec())
			}
			tr.unitTargets[unit.ID] = target
			tr.updateUnitEquipment(unit)
			tr.updateUnitAppearance(unit)
		} else {
			drawing.shaderData.Deactivate()
			delete(tr.unitTargets, unit.ID)
			delete(tr.unitPositions, unit.ID)
			tr.clearUnitEquipment(unit.ID)
			tr.clearUnitAppearance(unit.ID)
		}
	}

	for id, drawing := range tr.unitDrawings {
		if _, ok := seen[id]; !ok {
			drawing.shaderData.Deactivate()
			delete(tr.unitTargets, id)
			delete(tr.unitPositions, id)
			tr.clearUnitEquipment(id)
			tr.clearUnitAppearance(id)
		}
	}
}

func (tr *TerrainRenderer) updateUnitEquipment(unit mapdata.UnitInstance) {
	previous := tr.unitEquipment[unit.ID]
	current := make(map[string]*unitEquipmentDrawing, len(unit.Inventory))
	unitScaleValue := unitScale(unit)
	for index, inventory := range unit.Inventory {
		item := inventory.Item
		key := fmt.Sprintf("unit:%d:inventory:%d:%d", unit.ID, index, item.Type.MatType)
		mesh := tr.entityMesh(itemMeshKind(item))
		drawing := tr.ensureEntityDrawingWithMesh(key, unitEquipmentColor(unit, item), mesh)
		if drawing == nil {
			continue
		}
		offset := unitEquipmentOffset(index, inventory.Mode)
		itemSize, itemHeight := itemDimensions(item)
		current[key] = &unitEquipmentDrawing{
			drawing: drawing,
			offset:  offset,
			scale: unitPosition{
				x: itemSize * unitScaleValue,
				y: itemHeight * unitScaleValue,
				z: itemSize * unitScaleValue,
			},
		}
	}

	for key, old := range previous {
		if _, stillPresent := current[key]; !stillPresent {
			old.drawing.shaderData.Deactivate()
			delete(tr.entityDrawings, key)
		}
	}
	tr.unitEquipment[unit.ID] = current
}

func (tr *TerrainRenderer) clearUnitEquipment(id int32) {
	for key, equipment := range tr.unitEquipment[id] {
		if equipment != nil && equipment.drawing != nil {
			equipment.drawing.shaderData.Deactivate()
		}
		delete(tr.entityDrawings, key)
	}
	delete(tr.unitEquipment, id)
}

func (tr *TerrainRenderer) updateUnitAppearance(unit mapdata.UnitInstance) {
	previous := tr.unitAppearance[unit.ID]
	current := make(map[string]*unitEquipmentDrawing, 5)
	unitScaleValue := unitScale(unit)

	addPart := func(key string, meshKind string, color matrix.Color, offset, scale unitPosition) {
		drawing := tr.ensureEntityDrawingWithMesh(key, color, tr.entityMesh(meshKind))
		if drawing == nil {
			return
		}
		current[key] = &unitEquipmentDrawing{drawing: drawing, offset: offset, scale: scale}
	}

	// O corpo principal continua sendo a cápsula; estas peças dão silhueta
	// estável às criaturas mesmo quando não há sprite/raw gráfico disponível.
	addPart(
		fmt.Sprintf("unit:%d:appearance:head", unit.ID),
		"sphere",
		appearanceColor(unit, 0, darkenColor(unitColor(unit), 0.9)),
		unitPosition{x: 0, y: 0.34 * unitScaleValue, z: 0},
		unitPosition{x: 0.22 * unitScaleValue, y: 0.22 * unitScaleValue, z: 0.22 * unitScaleValue},
	)

	if unit.Appearance.Hair.Length > 0 {
		hairSize := matrix.Float(0.16) + matrix.Float(minInt32(unit.Appearance.Hair.Length, 40))*matrix.Float(0.002)
		addPart(
			fmt.Sprintf("unit:%d:appearance:hair", unit.ID),
			"sphere",
			appearanceColor(unit, 1, matrix.NewColor(0.12, 0.07, 0.04, 1)),
			unitPosition{x: 0, y: 0.48 * unitScaleValue, z: 0},
			unitPosition{x: hairSize * unitScaleValue, y: 0.08 * unitScaleValue, z: hairSize * unitScaleValue},
		)
	}

	if unit.Appearance.Beard.Length > 0 || unit.Appearance.Moustache.Length > 0 {
		addPart(
			fmt.Sprintf("unit:%d:appearance:beard", unit.ID),
			"sphere",
			appearanceColor(unit, 1, matrix.NewColor(0.1, 0.06, 0.035, 1)),
			unitPosition{x: 0, y: 0.28 * unitScaleValue, z: -0.17 * unitScaleValue},
			unitPosition{x: 0.11 * unitScaleValue, y: 0.12 * unitScaleValue, z: 0.045 * unitScaleValue},
		)
	}

	woundIndex := 0
	for _, wound := range unit.Wounds {
		if !wound.SeveredPart && len(wound.Parts) == 0 {
			continue
		}
		if woundIndex >= 3 {
			break
		}
		key := fmt.Sprintf("unit:%d:appearance:wound:%d", unit.ID, woundIndex)
		offset := unitPosition{
			x: (matrix.Float(woundIndex%3) - 1) * matrix.Float(0.14) * unitScaleValue,
			y: matrix.Float(0.02+float32(woundIndex)*0.16) * unitScaleValue,
			z: -0.18 * unitScaleValue,
		}
		addPart(key, "sphere", matrix.NewColor(0.8, 0.06, 0.035, 0.9), offset, unitPosition{
			x: 0.055 * unitScaleValue, y: 0.055 * unitScaleValue, z: 0.025 * unitScaleValue,
		})
		woundIndex++
	}

	for key, old := range previous {
		if _, stillPresent := current[key]; !stillPresent {
			old.drawing.shaderData.Deactivate()
			delete(tr.entityDrawings, key)
		}
	}
	tr.unitAppearance[unit.ID] = current
}

func (tr *TerrainRenderer) clearUnitAppearance(id int32) {
	for key, part := range tr.unitAppearance[id] {
		if part != nil && part.drawing != nil {
			part.drawing.shaderData.Deactivate()
		}
		delete(tr.entityDrawings, key)
	}
	delete(tr.unitAppearance, id)
}

func appearanceColor(unit mapdata.UnitInstance, index int, fallback matrix.Color) matrix.Color {
	if index >= 0 && index < len(unit.Appearance.Colors) {
		colorIndex := unit.Appearance.Colors[index]
		if colorIndex >= 0 && colorIndex < int32(len(mapdata.DFColorList)) {
			color := mapdata.DFColorList[colorIndex]
			return matrix.NewColor(
				matrix.Float(color.R)/255,
				matrix.Float(color.G)/255,
				matrix.Float(color.B)/255,
				1,
			)
		}
	}
	return fallback
}

func minInt32(value, max int32) int32 {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func unitEquipmentOffset(index int, mode int32) unitPosition {
	side := matrix.Float(-1)
	if index%2 == 1 {
		side = 1
	}
	return unitPosition{
		x: side * (matrix.Float(0.22) + matrix.Float(index/2)*matrix.Float(0.04)),
		y: matrix.Float(0.05) + matrix.Float(index%3)*matrix.Float(0.1),
		z: matrix.Float(0.08) + matrix.Float(mode&1)*matrix.Float(0.03),
	}
}

func unitEquipmentColor(unit mapdata.UnitInstance, item dfproto.Item) matrix.Color {
	if item.Dye.Red != 0 || item.Dye.Green != 0 || item.Dye.Blue != 0 {
		return matrix.NewColor(
			matrix.Float(item.Dye.Red)/255,
			matrix.Float(item.Dye.Green)/255,
			matrix.Float(item.Dye.Blue)/255,
			1,
		)
	}
	return darkenColor(unitColor(unit), 0.78)
}

func unitColor(unit mapdata.UnitInstance) matrix.Color {
	color := unit.ProfessionColor
	if color.Red == 0 && color.Green == 0 && color.Blue == 0 {
		if unit.IsSoldier {
			return matrix.NewColor(0.95, 0.55, 0.18, 1)
		}
		return matrix.NewColor(0.75, 0.9, 1.0, 1)
	}
	result := matrix.NewColor(
		matrix.Float(color.Red)/255,
		matrix.Float(color.Green)/255,
		matrix.Float(color.Blue)/255,
		1,
	)
	if unit.IsSoldier {
		// Um pequeno reforço quente mantém soldados distinguíveis mesmo quando
		// a cor de profissão recebida pelo DFHack é muito escura.
		result = matrix.NewColor(
			matrix.Float(math.Min(1, float64(result.R()+0.16))),
			matrix.Float(math.Min(1, float64(result.G()+0.06))),
			result.B(),
			1,
		)
	}
	return result
}

func unitScale(unit mapdata.UnitInstance) matrix.Float {
	if unit.SizeCurrent <= 0 || unit.SizeBase <= 0 {
		return 1
	}
	ratio := matrix.Float(unit.SizeCurrent) / matrix.Float(unit.SizeBase)
	if ratio < 0.65 {
		return 0.65
	}
	if ratio > 1.45 {
		return 1.45
	}
	return ratio
}

// UpdateChunkEntities atualiza construções e itens que acompanham um chunk.
// Eles usam desenhos persistentes e apenas mudam de posição/escala quando o
// snapshot do DFHack é renovado.
func (tr *TerrainRenderer) UpdateChunkEntities(chunk *mapdata.Chunk) {
	if chunk == nil {
		return
	}
	origin := chunk.Origin
	snapshot := chunk.Snapshot()
	key := origin.String()
	tr.queueMu.Lock()
	tr.chunkVersions[key]++
	delete(tr.invalidated, key)
	tr.pendingEntities[key] = chunkEntityUpdate{origin: origin, snapshot: snapshot}
	shouldQueue := !tr.entityFlushQueued
	if shouldQueue {
		tr.entityFlushQueued = true
	}
	tr.queueMu.Unlock()

	if shouldQueue {
		tr.host.RunOnMainThread(tr.flushEntityUpdates)
	}
}

func (tr *TerrainRenderer) flushEntityUpdates() {
	const maxUpdatesPerFrame = 8
	updates := make([]chunkEntityUpdate, 0, maxUpdatesPerFrame)

	tr.queueMu.Lock()
	for key, update := range tr.pendingEntities {
		delete(tr.pendingEntities, key)
		updates = append(updates, update)
		if len(updates) == maxUpdatesPerFrame {
			break
		}
	}
	more := len(tr.pendingEntities) > 0
	if !more {
		tr.entityFlushQueued = false
	}
	tr.queueMu.Unlock()

	for _, update := range updates {
		tr.applyChunkEntities(update.origin, update.snapshot)
	}
	if more {
		tr.host.RunAfterFrames(1, tr.flushEntityUpdates)
	}
}

func (tr *TerrainRenderer) ensureEntityResources() bool {
	if tr.unitMesh == nil {
		cache := tr.host.MeshCache()
		tr.entityMeshes["cube"] = rendering.NewMeshCube(cache)
		tr.entityMeshes["unit"] = rendering.NewMeshCapsule(cache, 0.5, 1.0, 12, 6)
		tr.entityMeshes["sphere"] = rendering.NewMeshSphere(cache, 0.5, 12, 16)
		tr.entityMeshes["cylinder"] = rendering.NewMeshCylinder(cache, 1.0, 0.5, 12, true)
		tr.entityMeshes["cone"] = rendering.NewMeshCone(cache, 1.0, 0.5, 12, true)
		tr.unitMesh = tr.entityMeshes["unit"]
		tr.unitMaterial, _ = tr.host.MaterialCache().Material("basic.material")
	}
	return tr.unitMesh != nil && tr.unitMaterial != nil
}

func (tr *TerrainRenderer) ensureTerrainMaterial() *rendering.Material {
	if tr.terrainMaterial != nil {
		return tr.terrainMaterial
	}
	base, err := tr.host.MaterialCache().Material("basic.material")
	if err != nil || base == nil {
		return base
	}

	const textureSize = 64
	textureData := makeDetailTexture(textureSize, textureSize)
	texture, err := tr.host.TextureCache().InsertRawTexture(
		"fv_terrain_detail_rgba",
		textureData,
		textureSize,
		textureSize,
		rendering.TextureFilterLinear,
	)
	if err != nil || texture == nil {
		return base
	}
	tr.terrainMaterial = base.CreateInstance([]*rendering.Texture{texture})
	return tr.terrainMaterial
}

func (tr *TerrainRenderer) ensureLiquidMaterial() *rendering.Material {
	if tr.liquidMaterial != nil {
		return tr.liquidMaterial
	}
	material, err := tr.host.MaterialCache().Material("basic_transparent.material")
	if err != nil || material == nil {
		return nil
	}
	tr.liquidMaterial = material
	return material
}

func (tr *TerrainRenderer) ensureEntityDrawing(key string, color matrix.Color) *unitDrawing {
	return tr.ensureEntityDrawingWithMesh(key, color, tr.entityMeshes["cube"])
}

func (tr *TerrainRenderer) ensureEntityDrawingWithMesh(key string, color matrix.Color, mesh *rendering.Mesh) *unitDrawing {
	return tr.ensureEntityDrawingWithMeshMaterial(key, color, mesh, tr.unitMaterial)
}

func (tr *TerrainRenderer) ensureEntityDrawingWithMeshMaterial(key string, color matrix.Color, mesh *rendering.Mesh, material *rendering.Material) *unitDrawing {
	if mesh == nil {
		mesh = tr.unitMesh
	}
	if material == nil {
		material = tr.unitMaterial
	}
	if drawing, ok := tr.entityDrawings[key]; ok {
		if standard, ok := drawing.shaderData.(*shader_data_registry.ShaderDataStandard); ok {
			standard.Color = color
		}
		drawing.shaderData.Activate()
		return drawing
	}

	sd := shader_data_registry.Create("basic")
	if sd == nil {
		return nil
	}
	if standard, ok := sd.(*shader_data_registry.ShaderDataStandard); ok {
		standard.Color = color
	}
	transform := &matrix.Transform{}
	transform.SetupRawTransform()
	drawing := &unitDrawing{transform: transform, shaderData: sd}
	tr.entityDrawings[key] = drawing
	tr.host.Drawings.AddDrawing(rendering.Drawing{
		Mesh:       mesh,
		Material:   material,
		ShaderData: sd,
		Transform:  transform,
		ViewCuller: &CameraViewCuller{Camera: tr.host.PrimaryCamera()},
	})
	return drawing
}

func (tr *TerrainRenderer) entityMesh(kind string) *rendering.Mesh {
	if mesh := tr.entityMeshes[kind]; mesh != nil {
		return mesh
	}
	return tr.entityMeshes["cube"]
}

func entityMaterialColor(snapshot mapdata.ChunkSnapshot, pair dfproto.MatPair, fallback matrix.Color) matrix.Color {
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if !snapshot.TilePresent[x][y] {
				continue
			}
			tile := &snapshot.Tiles[x][y]
			if tile.GetStore() != nil && tile.GetStore().MatStore != nil {
				if color, ok := tile.GetStore().MatStore.GetMaterialColor(pair); ok {
					return matrix.NewColor(
						matrix.Float(color.R)/255,
						matrix.Float(color.G)/255,
						matrix.Float(color.B)/255,
						matrix.Float(color.A)/255,
					)
				}
			}
		}
	}
	return fallback
}

func darkenColor(color matrix.Color, factor matrix.Float) matrix.Color {
	return matrix.NewColor(color.R()*factor, color.G()*factor, color.B()*factor, color.A())
}

func designationColor(designation dfproto.TileDigDesignation, marker, auto bool) matrix.Color {
	if marker {
		return matrix.NewColor(1, 0.15, 0.08, 1)
	}
	if auto {
		return matrix.NewColor(0.95, 0.65, 0.12, 1)
	}
	switch designation {
	case dfproto.DigChannel:
		return matrix.NewColor(0.1, 0.8, 1, 1)
	case dfproto.DigRamp:
		return matrix.NewColor(0.75, 0.25, 1, 1)
	case dfproto.DigUpStair, dfproto.DigDownStair, dfproto.DigUpDownStair:
		return matrix.NewColor(0.3, 0.55, 1, 1)
	default:
		return matrix.NewColor(1, 0.72, 0.1, 1)
	}
}

func artImageElementColor(snapshot mapdata.ChunkSnapshot, element dfproto.ArtImageElement) matrix.Color {
	fallback := matrix.NewColor(0.75, 0.75, 0.78, 1)
	switch element.Type {
	case dfproto.ImageCreature:
		fallback = matrix.NewColor(0.9, 0.35, 0.3, 1)
	case dfproto.ImagePlant:
		fallback = matrix.NewColor(0.35, 0.85, 0.3, 1)
	case dfproto.ImageTree:
		fallback = matrix.NewColor(0.55, 0.28, 0.12, 1)
	case dfproto.ImageShape:
		fallback = matrix.NewColor(0.7, 0.35, 0.9, 1)
	case dfproto.ImageItem:
		fallback = matrix.NewColor(1.0, 0.75, 0.25, 1)
	}
	if element.Material != (dfproto.MatPair{}) {
		return entityMaterialColor(snapshot, element.Material, fallback)
	}
	return fallback
}

func (tr *TerrainRenderer) applyChunkEntities(origin util.DFCoord, snapshot mapdata.ChunkSnapshot) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.ensureEntityResources() {
		return
	}

	chunkKey := origin.String()
	seen := make(map[string]struct{}, len(snapshot.Buildings)+len(snapshot.Items)+len(snapshot.OceanWaves))
	for _, building := range snapshot.Buildings {
		key := fmt.Sprintf("building:%d", building.Index)
		seen[key] = struct{}{}
		drawing := tr.ensureEntityDrawing(key, entityMaterialColor(snapshot, building.Material, matrix.NewColor(0.55, 0.55, 0.6, 1)))
		if drawing == nil {
			continue
		}
		width := matrix.Float(building.PosXMax - building.PosXMin + 1)
		depth := matrix.Float(building.PosYMax - building.PosYMin + 1)
		height := matrix.Float(building.PosZMax - building.PosZMin + 1)
		if width < 1 {
			width = 1
		}
		if depth < 1 {
			depth = 1
		}
		if height < 0.25 {
			height = 0.25
		}
		drawing.transform.SetPosition(matrix.NewVec3(
			matrix.Float(building.PosXMin)+width*0.5,
			matrix.Float(building.PosZMin)+height*0.5,
			-(matrix.Float(building.PosYMin) + depth*0.5),
		))
		drawing.transform.SetScale(matrix.NewVec3(width, height, depth))

		roofKey := fmt.Sprintf("building:%d:roof", building.Index)
		seen[roofKey] = struct{}{}
		roof := tr.ensureEntityDrawing(roofKey, darkenColor(entityMaterialColor(snapshot, building.Material, matrix.NewColor(0.55, 0.55, 0.6, 1)), 0.72))
		if roof != nil {
			roof.transform.SetPosition(matrix.NewVec3(
				matrix.Float(building.PosXMin)+width*0.5,
				matrix.Float(building.PosZMin)+height+0.06,
				-(matrix.Float(building.PosYMin) + depth*0.5),
			))
			roof.transform.SetScale(matrix.NewVec3(width*0.92, 0.12, depth*0.92))
		}

		for itemIndex, buildingItem := range building.Items {
			if buildingItem.Item == nil {
				continue
			}
			item := *buildingItem.Item
			itemKind := itemMeshKind(item)
			key := fmt.Sprintf("building:%d:item:%d:%s", building.Index, itemIndex, itemKind)
			seen[key] = struct{}{}
			itemColor := entityMaterialColor(snapshot, item.Material, darkenColor(entityMaterialColor(snapshot, building.Material, matrix.NewColor(0.6, 0.6, 0.65, 1)), 1.1))
			detail := tr.ensureEntityDrawingWithMesh(key, itemColor, tr.entityMesh(itemKind))
			if detail == nil {
				continue
			}
			itemPos := item.Pos
			if itemPos == (dfproto.Coord{}) {
				itemPos = dfproto.Coord{
					X: (building.PosXMin + building.PosXMax) / 2,
					Y: (building.PosYMin + building.PosYMax) / 2,
					Z: building.PosZMin,
				}
			}
			pos := util.DFToWorldPos(util.DFCoord{X: itemPos.X, Y: itemPos.Y, Z: itemPos.Z})
			itemSize, itemHeight := itemDimensions(item)
			detail.transform.SetPosition(matrix.NewVec3(
				matrix.Float(pos.X)+0.5+matrix.Float(item.SubposX),
				tileSurfaceY(matrix.Float(pos.Y), itemHeight*0.5+matrix.Float(item.SubposZ)),
				matrix.Float(pos.Z)-0.5-matrix.Float(item.SubposY),
			))
			detail.transform.SetScale(itemVisualScale(item, itemSize, itemHeight))
		}
	}

	for _, item := range snapshot.Items {
		itemKind := itemMeshKind(item)
		key := fmt.Sprintf("item:%d:%s", item.ID, itemKind)
		seen[key] = struct{}{}
		drawing := tr.ensureEntityDrawingWithMesh(key, entityMaterialColor(snapshot, item.Material, matrix.NewColor(1, 0.75, 0.2, 1)), tr.entityMesh(itemKind))
		if drawing == nil {
			continue
		}
		pos := util.DFToWorldPos(util.DFCoord{X: item.Pos.X, Y: item.Pos.Y, Z: item.Pos.Z})
		size, stackHeight := itemDimensions(item)
		drawing.transform.SetPosition(matrix.NewVec3(
			matrix.Float(pos.X)+0.5+matrix.Float(item.SubposX),
			tileSurfaceY(matrix.Float(pos.Y), stackHeight*0.5+matrix.Float(item.SubposZ)),
			matrix.Float(pos.Z)-0.5-matrix.Float(item.SubposY),
		))
		drawing.transform.SetScale(itemVisualScale(item, size, stackHeight))
	}

	for _, flow := range snapshot.Flows {
		if flow.Dead {
			continue
		}
		key := fmt.Sprintf("flow:%s:%d:%d:%d:%d", chunkKey, flow.Index, flow.Pos.X, flow.Pos.Y, flow.Pos.Z)
		seen[key] = struct{}{}
		color := flowColor(flow.Type)
		if flow.Material != (dfproto.MatPair{}) {
			color = entityMaterialColor(snapshot, flow.Material, color)
		}
		flowMaterial := tr.ensureLiquidMaterial()
		if flowMaterial == nil {
			flowMaterial = tr.unitMaterial
		}
		drawing := tr.ensureEntityDrawingWithMeshMaterial(key, color, tr.entityMesh("sphere"), flowMaterial)
		if drawing == nil {
			continue
		}
		pos := util.DFToWorldPos(util.DFCoord{X: flow.Pos.X, Y: flow.Pos.Y, Z: flow.Pos.Z})
		size := flowVisualSize(flow)
		drawing.transform.SetPosition(matrix.NewVec3(
			matrix.Float(pos.X)+0.5,
			matrix.Float(pos.Y)+0.55,
			matrix.Float(pos.Z)-0.5,
		))
		drawing.transform.SetScale(matrix.NewVec3(size, size, size))
	}

	// OceanWaves chegam no envelope do BlockList do RemoteFortressReader. Elas
	// representam a frente da onda entre Pos e Dest; um marcador achatado e
	// alongado mantém o efeito legível sem criar uma malha por tile.
	for _, wave := range snapshot.OceanWaves {
		key := fmt.Sprintf("wave:%s:%d:%d:%d:%d:%d:%d", chunkKey,
			wave.Pos.X, wave.Pos.Y, wave.Pos.Z, wave.Dest.X, wave.Dest.Y, wave.Dest.Z)
		seen[key] = struct{}{}
		waveMaterial := tr.ensureLiquidMaterial()
		if waveMaterial == nil {
			waveMaterial = tr.unitMaterial
		}
		drawing := tr.ensureEntityDrawingWithMeshMaterial(key, waveColor(), tr.entityMesh("sphere"), waveMaterial)
		if drawing == nil {
			continue
		}
		pos := util.DFToWorldPos(util.DFCoord{
			X: (wave.Pos.X + wave.Dest.X) / 2,
			Y: (wave.Pos.Y + wave.Dest.Y) / 2,
			Z: (wave.Pos.Z + wave.Dest.Z) / 2,
		})
		drawing.transform.SetPosition(matrix.NewVec3(
			matrix.Float(pos.X)+0.5,
			matrix.Float(pos.Y)+0.56,
			matrix.Float(pos.Z)-0.5,
		))
		drawing.transform.SetScale(waveVisualScale(wave))
	}

	for _, engraving := range snapshot.Engravings {
		if engraving.Hidden {
			continue
		}
		key := fmt.Sprintf("engraving:%d:%d:%d", engraving.Pos.X, engraving.Pos.Y, engraving.Pos.Z)
		seen[key] = struct{}{}
		quality := matrix.Float(engraving.Quality)
		if quality < 0 {
			quality = 0
		}
		if quality > 5 {
			quality = 5
		}
		engravingColor := matrix.NewColor(0.72+quality*0.045, 0.68+quality*0.045, 0.48+quality*0.05, 1)
		drawing := tr.ensureEntityDrawing(key, engravingColor)
		if drawing == nil {
			continue
		}
		pos := util.DFToWorldPos(util.DFCoord{X: engraving.Pos.X, Y: engraving.Pos.Y, Z: engraving.Pos.Z})
		if engraving.IsFloor {
			drawing.transform.SetPosition(matrix.NewVec3(
				matrix.Float(pos.X)+0.5,
				tileSurfaceY(matrix.Float(pos.Y), tileOverlayEpsilon+tileOverlayThickness*0.5),
				matrix.Float(pos.Z)-0.5,
			))
			drawing.transform.SetScale(matrix.NewVec3(0.56, tileOverlayThickness, 0.56))
		} else {
			drawing.transform.SetPosition(matrix.NewVec3(
				matrix.Float(pos.X)+0.5,
				matrix.Float(pos.Y)+0.5,
				matrix.Float(pos.Z)-0.985,
			))
			drawing.transform.SetScale(matrix.NewVec3(0.7, 0.7, 0.02))
		}

		for elementIndex, element := range engraving.Image.Elements {
			count := minInt32(element.Count, 4)
			if count < 1 {
				count = 1
			}
			for mark := int32(0); mark < count; mark++ {
				key := fmt.Sprintf("engraving:%d:%d:%d:image:%d:%d", engraving.Pos.X, engraving.Pos.Y, engraving.Pos.Z, elementIndex, mark)
				seen[key] = struct{}{}
				marker := tr.ensureEntityDrawing(key, artImageElementColor(snapshot, element))
				if marker == nil {
					continue
				}
				spread := matrix.Float(mark) - matrix.Float(count-1)*matrix.Float(0.5)
				if engraving.IsFloor {
					marker.transform.SetPosition(matrix.NewVec3(
						matrix.Float(pos.X)+0.5+spread*0.14,
						tileSurfaceY(matrix.Float(pos.Y), tileOverlayEpsilon+0.025),
						matrix.Float(pos.Z)-0.5+matrix.Float(elementIndex%3-1)*0.12,
					))
					marker.transform.SetScale(matrix.NewVec3(0.055, 0.018, 0.055))
				} else {
					marker.transform.SetPosition(matrix.NewVec3(
						matrix.Float(pos.X)+0.5+spread*0.14,
						matrix.Float(pos.Y)+0.5+matrix.Float(elementIndex%3-1)*0.12,
						matrix.Float(pos.Z)-0.965,
					))
					marker.transform.SetScale(matrix.NewVec3(0.07, 0.07, 0.025))
				}
			}
		}
	}

	// O RemoteFortressReader transmite o SpatterPile alinhado ao índice do
	// tile quando a lista possui os 256 elementos do bloco.
	if len(snapshot.SpatterPile) == 16*16 {
		for index, pile := range snapshot.SpatterPile {
			if len(pile.Spatters) == 0 {
				continue
			}
			x, y := index%16, index/16
			key := fmt.Sprintf("spatter:%s:%d", chunkKey, index)
			seen[key] = struct{}{}
			amount := int32(0)
			for _, spatter := range pile.Spatters {
				amount += spatter.Amount
			}
			spatterHeight := matrix.Float(0.008) + matrix.Float(amount)/2500
			if spatterHeight > 0.06 {
				spatterHeight = 0.06
			}
			spatterColor := entityMaterialColor(snapshot, pile.Spatters[0].Material, matrix.NewColor(0.45, 0.08, 0.04, 1))
			drawing := tr.ensureEntityDrawing(key, spatterColor)
			if drawing == nil {
				continue
			}
			pos := util.DFToWorldPos(util.DFCoord{X: origin.X + int32(x), Y: origin.Y + int32(y), Z: origin.Z})
			drawing.transform.SetPosition(matrix.NewVec3(
				matrix.Float(pos.X)+0.5,
				tileSurfaceY(matrix.Float(pos.Y), spatterHeight*0.5),
				matrix.Float(pos.Z)-0.5,
			))
			drawing.transform.SetScale(matrix.NewVec3(0.22, spatterHeight, 0.22))
		}
	}

	// Marcações de escavação são overlays de estado e não alteram a malha
	// permanente do piso. Mantê-las visíveis ajuda a conferir o mapa no 3D.
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if !snapshot.TilePresent[x][y] {
				continue
			}
			tile := snapshot.Tiles[x][y]
			if tile.DigDesignation == dfproto.DigNone && !tile.DigMarker && !tile.DigAuto {
				continue
			}
			key := fmt.Sprintf("designation:%s:%d", chunkKey, x+y*16)
			seen[key] = struct{}{}
			drawing := tr.ensureEntityDrawing(key, designationColor(tile.DigDesignation, tile.DigMarker, tile.DigAuto))
			if drawing == nil {
				continue
			}
			pos := util.DFToWorldPos(util.DFCoord{X: origin.X + int32(x), Y: origin.Y + int32(y), Z: origin.Z})
			drawing.transform.SetPosition(matrix.NewVec3(
				matrix.Float(pos.X)+0.5,
				tileSurfaceY(matrix.Float(pos.Y), tileOverlayEpsilon+tileOverlayThickness*0.5),
				matrix.Float(pos.Z)-0.5,
			))
			drawing.transform.SetScale(matrix.NewVec3(0.38, tileOverlayThickness, 0.38))
		}
	}

	previous := tr.chunkEntityKeys[chunkKey]
	for key := range previous {
		if _, ok := seen[key]; ok {
			continue
		}
		refs := tr.entityKeyRefs[key]
		delete(refs, chunkKey)
		if len(refs) == 0 {
			delete(tr.entityKeyRefs, key)
			if drawing, exists := tr.entityDrawings[key]; exists {
				drawing.shaderData.Deactivate()
			}
		}
	}
	for key := range seen {
		refs := tr.entityKeyRefs[key]
		if refs == nil {
			refs = make(map[string]struct{})
			tr.entityKeyRefs[key] = refs
		}
		refs[chunkKey] = struct{}{}
	}
	tr.chunkEntityKeys[chunkKey] = seen
}

func itemDimensions(item dfproto.Item) (matrix.Float, matrix.Float) {
	size := matrix.Float(0.16)
	if item.StackSize > 1 {
		size = 0.2
	}
	stackHeight := matrix.Float(0.14)
	if item.StackSize > 1 {
		stackHeight += matrix.Float(item.StackSize) * 0.018
		if stackHeight > 0.55 {
			stackHeight = 0.55
		}
	}
	return size, stackHeight
}

// itemVisualScale traduz dados físicos do RemoteFortressReader em uma forma
// simples: projéteis ficam alongados conforme a velocidade e volumes grandes
// ganham presença, sem alterar a posição do tile de origem.
func itemVisualScale(item dfproto.Item, size, height matrix.Float) matrix.Vec3 {
	if item.Projectile {
		velocityX := matrix.Abs(matrix.Float(item.VelocityX))
		velocityY := matrix.Abs(matrix.Float(item.VelocityY))
		velocityZ := matrix.Abs(matrix.Float(item.VelocityZ))
		if velocityX > 4 {
			velocityX = 4
		}
		if velocityY > 4 {
			velocityY = 4
		}
		if velocityZ > 4 {
			velocityZ = 4
		}
		size += (velocityX + velocityY) * matrix.Float(0.025)
		height += velocityZ * matrix.Float(0.025)
		if height < matrix.Float(0.16) {
			height = matrix.Float(0.16)
		}
	}
	return matrix.NewVec3(size, height, size)
}

func itemMeshKind(item dfproto.Item) string {
	if item.Projectile {
		return "cone"
	}
	if item.Volume > 0 && item.StackSize <= 1 {
		return "cylinder"
	}
	if item.StackSize > 1 {
		return "cube"
	}
	return "sphere"
}

// onMeshGenerated lida com novas geometrias vindas do mesher assíncrono.
// Cada chunk mantém uma malha dinâmica por submesh. Atualizações normais
// apenas reusam o desenho e enviam novos vértices ao GPU, sem destruir uma
// malha que o frame atual ainda pode estar usando.
func flowColor(flowType dfproto.FlowType) matrix.Color {
	switch flowType {
	case dfproto.FlowMiasma:
		return matrix.NewColor(0.35, 0.55, 0.18, 0.72)
	case dfproto.FlowSteam:
		return matrix.NewColor(0.88, 0.92, 0.95, 0.68)
	case dfproto.FlowMist:
		return matrix.NewColor(0.45, 0.72, 0.92, 0.62)
	case dfproto.FlowMaterialDust:
		return matrix.NewColor(0.72, 0.48, 0.25, 0.78)
	case dfproto.FlowMagmaMist:
		return matrix.NewColor(1.0, 0.28, 0.05, 0.82)
	case dfproto.FlowSmoke:
		return matrix.NewColor(0.23, 0.25, 0.28, 0.78)
	case dfproto.FlowDragonfire, dfproto.FlowFire, dfproto.FlowCampFire:
		return matrix.NewColor(1.0, 0.24, 0.03, 0.9)
	case dfproto.FlowWeb:
		return matrix.NewColor(0.88, 0.88, 0.76, 0.9)
	case dfproto.FlowMaterialGas, dfproto.FlowMaterialVapor:
		return matrix.NewColor(0.62, 0.38, 0.78, 0.72)
	case dfproto.FlowOceanWave, dfproto.FlowSeaFoam:
		return matrix.NewColor(0.30, 0.75, 0.95, 0.78)
	case dfproto.FlowItemCloud:
		return matrix.NewColor(0.86, 0.55, 0.25, 0.78)
	default:
		return matrix.NewColor(0.62, 0.62, 0.68, 0.7)
	}
}

func flowVisualSize(flow dfproto.FlowInfo) matrix.Float {
	density := matrix.Float(flow.Density)
	if density < 1 {
		density = 1
	}
	if density > 100 {
		density = 100
	}
	return matrix.Float(0.12) + density/matrix.Float(100)*matrix.Float(0.28)
}

func waveColor() matrix.Color {
	return matrix.NewColor(0.22, 0.78, 1.0, 0.82)
}

func waveVisualScale(wave dfproto.Wave) matrix.Vec3 {
	spanX := matrix.Abs(matrix.Float(wave.Dest.X - wave.Pos.X))
	spanY := matrix.Abs(matrix.Float(wave.Dest.Y - wave.Pos.Y))
	if spanX > 4 {
		spanX = 4
	}
	if spanY > 4 {
		spanY = 4
	}
	return matrix.NewVec3(
		matrix.Float(0.16)+spanX*matrix.Float(0.07),
		matrix.Float(0.045),
		matrix.Float(0.16)+spanY*matrix.Float(0.07),
	)
}

func (tr *TerrainRenderer) onMeshGenerated(mesh *mesher.ChunkMesh) {
	if mesh == nil {
		return
	}
	key := mesh.Origin.String()
	tr.queueMu.Lock()
	if _, discarded := tr.invalidated[key]; discarded {
		tr.queueMu.Unlock()
		return
	}
	tr.pendingMeshes[key] = mesh
	shouldQueue := !tr.meshFlushQueued
	if shouldQueue {
		tr.meshFlushQueued = true
	}
	tr.queueMu.Unlock()

	if shouldQueue {
		tr.host.RunOnMainThread(tr.flushMeshUpdates)
	}
}

func (tr *TerrainRenderer) flushMeshUpdates() {
	const maxUpdatesPerFrame = 4
	meshes := make([]*mesher.ChunkMesh, 0, maxUpdatesPerFrame)

	tr.queueMu.Lock()
	for key, mesh := range tr.pendingMeshes {
		delete(tr.pendingMeshes, key)
		meshes = append(meshes, mesh)
		if len(meshes) == maxUpdatesPerFrame {
			break
		}
	}
	more := len(tr.pendingMeshes) > 0
	if !more {
		tr.meshFlushQueued = false
	}
	tr.queueMu.Unlock()

	for _, mesh := range meshes {
		tr.applyMeshGenerated(mesh)
	}
	if more {
		tr.host.RunAfterFrames(1, tr.flushMeshUpdates)
	}
}

func (tr *TerrainRenderer) applyMeshGenerated(mesh *mesher.ChunkMesh) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	key := mesh.Origin.String()
	states := tr.chunkDrawings[key]
	if states == nil {
		states = make(map[int32]*chunkDrawing)
		tr.chunkDrawings[key] = states
	}
	seen := make(map[int32]struct{}, len(mesh.SubMeshes))
	viewCuller := &CameraViewCuller{Camera: tr.host.PrimaryCamera()}

	for matID, data := range mesh.SubMeshes {
		if data == nil || len(data.Indices) < 3 {
			continue
		}
		seen[matID] = struct{}{}
		mat := tr.ensureTerrainMaterial()
		if matID == 1 {
			// Água, magma e setas de fluxo carregam alpha nos vértices. O
			// material opaco do terreno descartava esse alpha e transformava
			// líquidos em placas sólidas sobre o mapa.
			mat = tr.ensureLiquidMaterial()
			if mat == nil {
				mat = tr.ensureTerrainMaterial()
			}
		}
		if mat == nil {
			continue
		}
		triangleCount := len(data.Indices) / 3
		state := states[matID]
		if state != nil && triangleCount <= state.capacity {
			verts, _ := padMeshData(data, state.capacity, mesh.Origin)
			tr.host.MeshCache().UpdateMeshVertices(state.meshKey, verts)
			state.shaderData.Activate()
			continue
		}

		capacity := nextPowerOfTwo(triangleCount)
		verts, indices := padMeshData(data, capacity, mesh.Origin)
		meshKey := fmt.Sprintf("chunk_%s_%d_dynamic_%d", key, matID, capacity)
		dynamicMesh := tr.host.MeshCache().DynamicMesh(meshKey, verts, indices)
		sd := shader_data_registry.Create("basic")
		if sd == nil {
			continue
		}
		if standard, ok := sd.(*shader_data_registry.ShaderDataStandard); ok {
			standard.Color = matrix.ColorWhite()
		}
		transform := &matrix.Transform{}
		transform.SetupRawTransform()
		transform.SetPosition(mesh.Origin)
		newState := &chunkDrawing{
			mesh:       dynamicMesh,
			meshKey:    meshKey,
			capacity:   capacity,
			transform:  transform,
			shaderData: sd,
		}
		if state != nil {
			// Só ocorre quando a capacidade cresce. A antiga fica desativada,
			// sem invalidar o frame que ainda aponta para ela.
			state.shaderData.Deactivate()
		}
		states[matID] = newState
		tr.host.Drawings.AddDrawing(rendering.Drawing{
			Mesh:       dynamicMesh,
			Material:   mat,
			ShaderData: sd,
			Transform:  transform,
			ViewCuller: viewCuller,
		})
	}

	for matID, state := range states {
		if _, ok := seen[matID]; !ok {
			state.shaderData.Deactivate()
		}
	}
}

// RemoveChunk desativa a geometria e as entidades do chunk quando o servidor
// informa que aquela coordenada é ar. Os objetos ficam no cache do engine para
// não invalidar referências de um frame em andamento, mas deixam de participar
// do desenho imediatamente.
func (tr *TerrainRenderer) RemoveChunk(origin util.DFCoord) {
	key := origin.String()
	tr.queueMu.Lock()
	tr.chunkVersions[key]++
	version := tr.chunkVersions[key]
	delete(tr.pendingMeshes, key)
	delete(tr.pendingEntities, key)
	tr.invalidated[key] = struct{}{}
	tr.queueMu.Unlock()
	tr.host.RunOnMainThread(func() {
		tr.queueMu.Lock()
		currentVersion := tr.chunkVersions[key]
		tr.queueMu.Unlock()
		if currentVersion != version {
			return
		}
		tr.mu.Lock()
		defer tr.mu.Unlock()

		for _, state := range tr.chunkDrawings[key] {
			state.shaderData.Deactivate()
		}
		delete(tr.chunkDrawings, key)

		for entityKey := range tr.chunkEntityKeys[key] {
			refs := tr.entityKeyRefs[entityKey]
			delete(refs, key)
			if len(refs) == 0 {
				delete(tr.entityKeyRefs, entityKey)
				if drawing, ok := tr.entityDrawings[entityKey]; ok {
					drawing.shaderData.Deactivate()
				}
			}
		}
		delete(tr.chunkEntityKeys, key)
	})
}

func nextPowerOfTwo(value int) int {
	if value <= 1 {
		return 1
	}
	result := 1
	for result < value {
		result <<= 1
	}
	return result
}

// padMeshData reserva uma quantidade fixa de triângulos. Os slots não usados
// viram triângulos degenerados e, portanto, não alteram a imagem.
func padMeshData(data *mesher.MeshData, capacity int, origin matrix.Vec3) ([]rendering.Vertex, []uint32) {
	if capacity < 1 {
		capacity = 1
	}
	verts := make([]rendering.Vertex, capacity*3)
	indices := make([]uint32, capacity*3)
	triangleCount := 0
	if data != nil {
		triangleCount = len(data.Indices) / 3
		if triangleCount > capacity {
			triangleCount = capacity
		}
	}

	var fallback rendering.Vertex
	if data != nil && len(data.Vertices) > 0 {
		fallback = data.Vertices[0]
	}
	for triangle := 0; triangle < capacity; triangle++ {
		base := triangle * 3
		if triangle < triangleCount && data != nil {
			for corner := 0; corner < 3; corner++ {
				sourceIndex := data.Indices[base+corner]
				if int(sourceIndex) < len(data.Vertices) {
					verts[base+corner] = data.Vertices[sourceIndex]
					verts[base+corner].UV0 = terrainDetailUV(verts[base+corner].Position, origin)
				} else {
					verts[base+corner] = fallback
				}
				indices[base+corner] = uint32(base + corner)
			}
			continue
		}
		verts[base], verts[base+1], verts[base+2] = fallback, fallback, fallback
		indices[base], indices[base+1], indices[base+2] = uint32(base), uint32(base), uint32(base)
	}
	return verts, indices
}

func terrainDetailUV(position, origin matrix.Vec3) matrix.Vec2 {
	return matrix.NewVec2(
		(position.X()+origin.X())*0.42,
		(position.Z()+origin.Z())*0.42,
	)
}

func makeDetailTexture(width, height int) []byte {
	if width < 1 || height < 1 {
		return nil
	}
	data := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := matrix.Float(0)
			amplitude := matrix.Float(0.13)
			normalization := matrix.Float(0)
			for octave := 0; octave < 3; octave++ {
				frequency := matrix.Float(1)
				for scale := 0; scale < octave; scale++ {
					frequency *= 2
				}
				phaseX := matrix.Float(x) * frequency / matrix.Float(width)
				phaseY := matrix.Float(y) * frequency / matrix.Float(height)
				value += (matrix.Float(math.Sin(float64(phaseX*17+phaseY*11)))*0.5 + 0.5) * amplitude
				normalization += amplitude
				amplitude *= 0.5
			}
			value /= normalization
			value = matrix.Clamp(0.82+value*0.22, 0.72, 1.0)
			channel := uint8(value * 255)
			index := (y*width + x) * 4
			data[index] = channel
			data[index+1] = channel
			data[index+2] = channel
			data[index+3] = 255
		}
	}
	return data
}

// Render mantém a API do controlador; os desenhos já são registrados quando a
// malha termina de ser gerada e permanecem persistentes na KaijuEngine.
func (tr *TerrainRenderer) Render() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// O UnitList chega em snapshots discretos. Um fator fixo por frame produz
	// uma interpolação estável sem depender da frequência das RPCs do DFHack.
	const interpolationAlpha matrix.Float = 0.18
	for id, target := range tr.unitTargets {
		current, ok := tr.unitPositions[id]
		if !ok {
			current = target
		}
		current = lerpUnitPosition(current, target, interpolationAlpha)
		if closeUnitPosition(current, target) {
			current = target
		}
		tr.unitPositions[id] = current
		if drawing := tr.unitDrawings[id]; drawing != nil {
			drawing.transform.SetPosition(current.vec())
		}
		for _, equipment := range tr.unitEquipment[id] {
			if equipment == nil || equipment.drawing == nil {
				continue
			}
			equipment.drawing.transform.SetPosition(addUnitPosition(current, equipment.offset).vec())
			equipment.drawing.transform.SetScale(equipment.scale.vec())
		}
		for _, part := range tr.unitAppearance[id] {
			if part == nil || part.drawing == nil {
				continue
			}
			part.drawing.transform.SetPosition(addUnitPosition(current, part.offset).vec())
			part.drawing.transform.SetScale(part.scale.vec())
		}
	}
}

func addUnitPosition(a, b unitPosition) unitPosition {
	return unitPosition{x: a.x + b.x, y: a.y + b.y, z: a.z + b.z}
}

func closeUnitPosition(a, b unitPosition) bool {
	const epsilon matrix.Float = 0.002
	return matrix.Abs(a.x-b.x) <= epsilon &&
		matrix.Abs(a.y-b.y) <= epsilon &&
		matrix.Abs(a.z-b.z) <= epsilon
}
