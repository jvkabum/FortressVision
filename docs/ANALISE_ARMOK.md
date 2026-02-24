# Análise de Replicação: Armok Vision vs FortressVision

Este documento detalha as lacunas técnicas e o plano de ação para atingir 100% de fidelidade visual com o **Armok Vision** original.

## 📐 Geometria e Meshing
- [ ] **Quinas Suavizadas (Voxels)**
  - *Armok*: Triangulação de células baseada em voxels (VoxelGenerator).
  - *FortressVision*: Blocos rígidos e Greedy Meshing.
  - *Ação*: Implementar `internal/meshing/voxel_mesher.go`.
- [ ] **Triangulação Orgânica**
  - *Armok*: Suporte a formas não-cúbicas via `Poly2Tri`.
  - *FortressVision*: Somente Cubos e Rampas.
  - *Ação*: Integrar biblioteca de triangulação (ex: `earcut-go`).
- [ ] **Meshing de Itens Dinâmicos**
  - *Armok*: Cria modelos 3D para itens no chão em tempo real.
  - *FortressVision*: Apenas blocos e líquidos.
  - *Ação*: Criar sistema de `item_mesher`.

## 🎨 Materiais e Shaders
- [ ] **Texture Arrays (Splatting)**
  - *Armok*: Mistura múltiplas texturas por tile (SplatMaps).
  - *FortressVision*: Uma textura/cor fixa por face.
  - *Ação*: Migrar para `Texture2DArray` e GLSL personalizado.
- [ ] **Contaminantes (Spatter)**
  - *Armok*: Camadas dinâmicas de sangue, lama e neve.
  - *FortressVision*: Não implementado.
  - *Ação*: Ler `tile.spatters` e gerar decais dinâmicos.
- [ ] **Efeitos de Fluxo Avançados**
  - *Armok*: Miasma, fumaça e fogo volumétrico.
  - *FortressVision*: Apenas líquidos básicos.
  - *Ação*: Portar logic de partículas e shaders do Armok.

## 🧚 Anatomia e Entidades (Avançado)
- [ ] **Construção Dinâmica de Corpos**
  - *Armok*: Reconstrói criaturas parte por parte (braços, pernas, órgãos) baseada nos Raws do DF.
  - *FortressVision*: Sprites estáticos ou nulos.
  - *Ação*: Criar sistema de `creature_body_builder` em Go.
- [ ] **Layering de Equipamento**
  - *Armok*: Renderiza roupas (camisas, calças, armaduras) em camadas sobre o corpo.
  - *FortressVision*: Não implementado.
  - *Ação*: Implementar sistema de camadas de mesh para unidades.
- [ ] **Sistema de Criaturas (Sprites)**
  - *Armok*: Billboard sprites com suporte a transparência e sombras.
  - *FortressVision*: Implementação básica legado.
  - *Ação*: Criar `CreatureSpriteManager`.
- [ ] **Interpolação de Movimento**
  - *Armok*: Movimento suave entre tiles usando `subpos`.
  - *FortressVision*: Movimento instantâneo (snap).
  - *Ação*: Implementar sistema de lerp de posição.

## ☀️ Ambiente e Ciclo de Tempo
- [ ] **Ciclo Celestial Sincronizado**
  - *Armok*: Sol e Lua movem-se conforme o relógio do DF (DFTime). Sincronizado com o tempo do jogo.
  - *FortressVision*: Iluminação estática.
  - *Ação*: Implementar conversão de ticks do DF para `SunAngle`.
- [ ] **Temperatura de Cor Dinâmica**
  - *Armok*: A cor da luz muda conforme a hora e estação do DF.
  - *FortressVision*: Não implementado.
  - *Ação*: Portar lógica de `ColorTemperature.cs` para o renderizador Raylib.

## 🖥️ Interface e Interação
- [ ] **Previews de Construção 3D**
  - *Armok*: Desenha transparências e cores (Verde/Vermelho/Roxo) para validar posicionamento de prédios.
  - *FortressVision*: Não implementado.
  - *Ação*: Criar shaders de preview e lógica de `BuildSelector`.
- [ ] **Sincronização de Sidebar**
  - *Armok*: Menus dinâmicos que espelham o estado do DF Hack Sidebar.
  - *FortressVision*: Não implementado.
  - *Ação*: Implementar watcher para `SidebarState`.

## 🛠️ Infraestrutura e Dados
- [x] **Conexão DFHack (RPC)**
- [x] **Gerenciamento de Cache**
- [x] **Sincronização de Mundo**

---
*Legenda: (x) Concluído | ( ) Pendente*
