# Progresso do Projeto: FortressVision v1 (Go)

> [!NOTE]
> Para detalhes sobre a paridade técnica e o plano de replicação do Armok Vision original, consulte a [Análise de Replicação](ANALISE_ARMOK.md).

Este documento rastreia o status da implementação do FortressVision em Go, baseado no mapeamento técnico do Armok Vision original.

## 🏗️ Core & Infraestrutura
- [x] Conexão gRPC/Protobuf com DFHack (`internal/dfhack`)
- [x] Gerenciamento de MapData em cache local (`internal/mapdata`)
- [x] Persistência com SQLite + GORM para carregamento offline
- [x] Sistema de Multithreading (Scanner e Mesher em goroutines)
- [x] Scanner Assíncrono para busca de novos blocos

## 📐 Geometria e Meshing
- [x] Algoritmo de Greedy Meshing (Otimização de faces)
- [x] Pool de Memória para buffers de mesh (`sync.Pool`)
- [x] Renderização de Terreno (Blocos opacos)
- [x] Renderização de Líquidos (Superfícies de Água/Magma)
- [ ] Implementação de Quinas Suavizadas (VoxelGenerator logic do Armok Vision)
- [ ] Meshing de Itens Dinâmicos
- [ ] Renderização de Gravuras (ArtImage e Verbos de Arte)
- [ ] Visualização de Veios de Minério e Eventos de Bloco

## 🎨 Materiais e Shaders
- [x] Renderização Básica com Raylib
- [x] Shaders de Fluxo para Água/Magma (Flowing Shaders)
- [ ] Sistema de Splatting de Materiais (Texture Arrays adaptados do DF)
- [ ] Suporte a Contaminantes (Sangue/Lama no terreno)
- [ ] Materiais específicos do DF (Cor, Metalicidade, Transparência)
- [ ] Paleta de Cores GPS (UCCcolor) para paridade visual
- [ ] Fluxos Avançados (Miasma, Fumaça, Fogo, Teias)
- [ ] Animação de Ondas Oceânicas (Ocean Waves)

## 🧚 Entidades e Ambientes
- [x] Câmera e Controles WASD/Mouse
- [ ] Construção Dinâmica de Corpos (Peça por peça)
- [ ] Layering de Equipamento (Vestuário em camadas)
- [ ] Renderização de Criaturas (Legacy Sprite Manager)
- [ ] Renderização de Itens (XML Mappings)
- [ ] Sistema de Vegetação e Crescimento de Plantas
- [ ] Sincronização de Ciclo Celestial (Sol/Lua via DFTime)
- [ ] Temperatura de Cor Dinâmica (Hora/Estação)
- [ ] Mapeamento de Zonas e Estoques (CivZones & Stockpiles)
- [ ] Sistema de Clima Avançado (Nuvens e Frentes)
- [ ] Previews de Construção 3D (Validação de local)
- [ ] Sincronização de Menus/Sidebar (Real-time sync)
- [ ] Renderização de Projéteis (Trajetórias e Velocidade)
- [ ] Suavização de Movimento de Unidades (Interpolation via `subpos`)
- [ ] Sincronização de Menus e Reports (Contextual Overlays)

## 🛠️ Manutenção e Build
- [x] Script de Build PowerShell (`build.ps1`)
- [x] Gerenciamento de Ícones e Recursos Windows
- [x] Sistema de Logs de Debug
- [x] Configuração via `config.json`
- [x] Engenharia de Contexto (PREVC .context em ambos os projetos)

---
*Legenda: (x) Concluído | ( ) Pendente*
