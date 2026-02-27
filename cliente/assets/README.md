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

---
*Nota: A maioria dos modelos foi migrada do projeto Armok Vision e organizada para máxima performance e compatibilidade com o motor Raylib (Go).*
