# Estrutura de Assets - FortressVision

Este diretório contém todos os recursos visuais utilizados pelo cliente FortressVision. A estrutura foi organizada para facilitar o consumo por categoria de objeto no Dwarf Fortress.

## 📂 Models (`assets/models/`)
Todos os modelos 3D nos formatos `.obj`, `.fbx` e `.stl`.

*   **`architecture/`**: Elementos de construção e móveis.
    *   Paredes, pisos, escadas, fortifications.
    *   Camas, mesas, oficinas, portas.
*   **`environment/`**: Elementos naturais e terreno.
    *   Árvores, troncos, galhos, arbustos, grama.
    *   Rampas, pedregulhos, bueiros.
*   **`items/`**: Objetos móveis e equipamentos.
    *   Barris, caixotes, jaulas, sacos.
    *   Armas, escudos, ferramentas.
    *   Gemas e joias.
*   **`mechanics/`**: Engenharia e dispositivos.
    *   Engrenagens, alavancas, placas de pressão, eixos.
*   **`units/`**: Representações de criaturas.
    *   Modelos de esqueletos e bases para anões.

## 📂 Textures (`assets/textures/`)
Texturas e mapas de bits organizados por uso.

*   **`blocks/`**: Texturas para o terreno e construções (pedra, grama, madeira, mármore).
*   **`items/`**: Texturas e ícones para itens (gemas, ferramentas).
*   **`entities/`**: Texturas para criaturas e unidades.
*   **`ui/`**: Elementos da interface do usuário.

## 👾 Sprites (`assets/sprites/`)
Imagens 2D (billboards) importadas do Armok Vision para atuar como recursos visuais alternativos ou substitutos de modelos 3D inexistentes.

*   `dwarf_male/`, `dwarf_female/`, `dwarf_child/`: Sprites de anões.
*   `human_male/`, `human_female/`: Sprites de humanos.
*   `animals_domestic/`, `animals_wild/`, `creatures_other/`: Fauna e monstros globais.
*   `items/`: Sprites 2D de comidas, sementes e ferramentas secundárias.
*   `ui_graphics/`: Tiles originais do jogo.

## ⚙️ Config (`assets/config/`)
Arquivos JSON de mapeamento entre tokens do Dwarf Fortress e modelos/texturas.

*   `tile_meshes.json`: Terreno (WALL, FLOOR, RAMP, TREE, etc.)
*   `building_meshes.json`: Construções (Bed, Chair, Door, Workshops, etc.)
*   `growth_meshes.json`: Folhagens, frutas, flores e palmeiras.
*   `designation_meshes.json`: Overlays de escavação e designação.
*   `collision_meshes.json`: Meshes simplificadas para colisão.
*   `tile_textures.json`: Normal/Occlusion maps por material (PBR).

## 🔊 Sons (`assets/sounds/`)
Efeitos sonoros básicos (passos, pulos).

## 📡 Proto (`assets/proto/`)
Definições Protobuf do DFHack para comunicação com o jogo.

## 🌐 Localização (`assets/localization/`)
Traduções da interface (pt-BR, en).

---
*Nota: Todos os recursos foram migrados do projeto Armok Vision e organizados para máxima performance e compatibilidade com o motor Raylib (Go).*
