# Referências de Arquitetura em Go para o FortressVision

Análise de repositórios em Go com soluções arquiteturais relevantes para os desafios atuais do projeto FortressVision (sincronia de chunks, geração de malhas 3D e testes).

## 1. `opd-ai/venture` (O gerador infinito RPG)
**Repositório:** [https://github.com/opd-ai/venture](https://github.com/opd-ai/venture)

Esse é um jogo Action-RPG 2D todo feito em Go (usando a engine *Ebiten*). O ponto forte do pacote `pkg/world` deles é a **Geração Procedural**.

*   **O que é:** Incrível para arquitetura de dados pura (ECS - Entity Component System). Eles lidam com "mundos infinitos" baseados em *seed*, usando algoritmos como *Autômatos Celulares* (para cavernas) e *Voronoi*.
*   **Como serve para o FortressVision:** A arquitetura de rede deles é desenhada para suportar alta latência e sincronia de "snapshots", o que é muito semelhante ao nosso fluxo de dados (DFHack -> Servidor -> Cliente 3D). Quando formos implementar a renderização visual e lógica de entidades móveis (Anões, Animais, Invasores), a arquitetura ECS deles é uma das melhores referências modernas em Go de como organizar a memória do jogo sem causar travamentos e gargalos de CPU.

## 2. `tehcyx/goengine` (O Parente Próximo)
**Repositório:** [https://github.com/tehcyx/goengine](https://github.com/tehcyx/goengine)

Essa é uma engine Voxel nativa em Go (utilizando OpenGL). Essencialmente, é o motor 3D de um jogo estilo Minecraft construído do zero.

*   **O que é:** Uma referência técnica pura de renderização voxel. Renderizar mundos em blocos de forma ingênua (desenhando cubo por cubo) sobrecarrega drasticamente a GPU devido ao número excessivo de polígonos nas faces não visíveis entre os blocos (inside faces).
*   **Como serve para o FortressVision:** Nosso sistema de construção de geometria bidimensional e tridimensional (o *Mesher*) atualmente agrupa e converte os blocos isolados do Dwarf Fortress para enviar à Raylib. O `goengine` aborda resoluções e implementações maduras de *Greedy Meshing* (um algoritmo sofisticado para mesclar faces texturizadas adjacentes do mesmo material, convertendo múltiplos cubos alinhados em apenas dois grandes polígonos - um 'quadrado' esticado). Para otimizar o cliente do FortressVision a ponto de rodar embarcações ou montanhas inteiras a 120 FPS constantes sem estresse da placa de vídeo, estudar o código de geração de malha desse projeto poupará semanas de pesadelo matemático e de otimização no CGO.

## 3. `gollilla/best` (O Testador Automatizado)
**Repositório:** [https://github.com/gollilla/best](https://github.com/gollilla/best)

A sigla significa *Bedrock Edition Server Testing*. Não é uma engine gráfica em si, mas sim um framework de testes automatizados projetado para plugar em um servidor de Minecraft (edição Bedrock).

*   **O que é:** Uma ferramenta analítica potente cujo pacote `pkg/world` é inteiramente focado em criar *Asserções de Mundo* (ex: ter a capacidade de perguntar de forma determinística à engine: "Pelo que foi injetado na memória, o bloco exatamente na coordenada [10, 50, -4] realmente tem as propriedades físicas de 'Terra'?").
*   **Como serve para o FortressVision:** O nosso Cliente do FortressVision atua, conceitualmente, de maneira similar a este testador: de forma passiva observando, iterando e reagindo aos eventos ditados puramente pelo estado da memória do jogo real e do DFHack (o "Servidor Autoritário"). À medida que a densidade do escopo cresce, prever a estabilidade contínua entre dezenas de atualizações tanto do cliente em Go, quanto do Raw Proto e do Dwarf Fortress, será complexo. Podemos nos inspirar no modus-operandi do `best` para compor uma suíte de Testes de Integração ou Simulação de Mundo em CI/CD: *"Finja injetar um bloco RAW com propriedades XYZ de Magma subindo através do servidor mock; verifique se o Mesher do Cliente instanciou os sprites e fluxos de lava no offset correto sem 'crashes'"*.

---

### Resumo Executivo
Estes três repositórios em Go lidam exatamente com os mesmos entraves estruturais primordiais que teremos nas etapas de polimento subsequentes do rendering 3D: propagação de rede e serialização em lotes eficientes num eixo infinito de blocos (`venture`), os mais recentes paradigmas de culling e algoritmos matemáticos enxutos para conversão Voxel/Malha (`goengine`), e testabilidade e injeção do paradigma client-server headless (`best`).
