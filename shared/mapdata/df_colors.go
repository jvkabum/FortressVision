package mapdata

// DFColor representa uma cor nomeada do Dwarf Fortress.
// Baseado em Assets/Scripts/DFColorList.cs do Armok Vision.
type DFColor struct {
	Token   string
	Name    string
	R, G, B uint8
}

// DFColorList é a tabela completa de 115 cores do Dwarf Fortress.
// Usada para mapear tokens de cor (ex: "AMBER", "IRON") para valores RGB.
var DFColorList = []DFColor{
	{"AMBER", "amber", 255, 191, 0},
	{"AMETHYST", "amethyst", 153, 102, 204},
	{"AQUA", "aqua", 0, 255, 255},
	{"AQUAMARINE", "aquamarine", 127, 255, 212},
	{"ASH_GRAY", "ash gray", 178, 190, 181},
	{"AUBURN", "auburn", 111, 53, 26},
	{"AZURE", "azure", 0, 127, 255},
	{"BEIGE", "beige", 245, 245, 220},
	{"BLACK", "black", 0, 0, 0},
	{"BLUE", "blue", 0, 0, 255},
	{"BRASS", "brass", 181, 166, 66},
	{"BRONZE", "bronze", 205, 127, 50},
	{"BROWN", "brown", 150, 75, 0},
	{"BUFF", "buff", 240, 220, 130},
	{"BURNT_SIENNA", "burnt sienna", 233, 116, 81},
	{"BURNT_UMBER", "burnt umber", 138, 51, 36},
	{"CARDINAL", "cardinal", 196, 30, 58},
	{"CARMINE", "carmine", 150, 0, 24},
	{"CERULEAN", "cerulean", 0, 123, 167},
	{"CHARCOAL", "charcoal", 54, 69, 79},
	{"CHARTREUSE", "chartreuse", 127, 255, 0},
	{"CHESTNUT", "chestnut", 205, 92, 92},
	{"CHOCOLATE", "chocolate", 210, 105, 30},
	{"CINNAMON", "cinnamon", 123, 63, 0},
	{"CLEAR", "clear", 128, 128, 128},
	{"COBALT", "cobalt", 0, 71, 171},
	{"COPPER", "copper", 184, 115, 51},
	{"CREAM", "cream", 255, 253, 208},
	{"CRIMSON", "crimson", 220, 20, 60},
	{"DARK_BLUE", "dark blue", 0, 0, 139},
	{"DARK_BROWN", "dark brown", 101, 67, 33},
	{"DARK_CHESTNUT", "dark chestnut", 152, 105, 96},
	{"DARK_GREEN", "dark green", 1, 50, 32},
	{"DARK_INDIGO", "dark indigo", 49, 0, 98},
	{"DARK_OLIVE", "dark olive", 85, 104, 50},
	{"DARK_PEACH", "dark peach", 255, 218, 185},
	{"DARK_PINK", "dark pink", 231, 84, 128},
	{"DARK_SCARLET", "dark scarlet", 86, 3, 25},
	{"DARK_TAN", "dark tan", 145, 129, 81},
	{"DARK_VIOLET", "dark violet", 66, 49, 137},
	{"ECRU", "ecru", 194, 178, 128},
	{"EGGPLANT", "eggplant", 97, 64, 81},
	{"EMERALD", "emerald", 80, 200, 120},
	{"FERN_GREEN", "fern green", 79, 121, 66},
	{"FLAX", "flax", 238, 220, 130},
	{"FUCHSIA", "fuchsia", 244, 0, 161},
	{"GOLD", "gold", 212, 175, 55},
	{"GOLDEN_YELLOW", "golden yellow", 255, 223, 0},
	{"GOLDENROD", "goldenrod", 218, 165, 32},
	{"GRAY", "gray", 128, 128, 128},
	{"GREEN", "green", 0, 255, 0},
	{"GREEN_YELLOW", "green-yellow", 173, 255, 47},
	{"HELIOTROPE", "heliotrope", 223, 115, 255},
	{"INDIGO", "indigo", 75, 0, 130},
	{"IVORY", "ivory", 255, 255, 240},
	{"JADE", "jade", 0, 168, 107},
	{"LAVENDER", "lavender", 230, 230, 250},
	{"LAVENDER_BLUSH", "lavender blush", 255, 240, 245},
	{"LEMON", "lemon", 253, 233, 16},
	{"LIGHT_BLUE", "light blue", 173, 216, 230},
	{"LIGHT_BROWN", "light brown", 205, 133, 63},
	{"LILAC", "lilac", 200, 162, 200},
	{"LIME", "lime", 204, 255, 0},
	{"MAHOGANY", "mahogany", 192, 64, 0},
	{"MAROON", "maroon", 128, 0, 0},
	{"MAUVE", "mauve", 153, 51, 102},
	{"MAUVE_TAUPE", "mauve taupe", 145, 95, 109},
	{"MIDNIGHT_BLUE", "midnight blue", 0, 51, 102},
	{"MINT_GREEN", "mint green", 152, 255, 152},
	{"MOSS_GREEN", "moss green", 173, 223, 173},
	{"OCHRE", "ochre", 204, 119, 34},
	{"OLIVE", "olive", 128, 128, 0},
	{"ORANGE", "orange", 255, 165, 0},
	{"PALE_BLUE", "pale blue", 175, 238, 238},
	{"PALE_BROWN", "pale brown", 152, 118, 84},
	{"PALE_CHESTNUT", "pale chestnut", 221, 173, 175},
	{"PALE_PINK", "pale pink", 250, 218, 221},
	{"PEACH", "peach", 255, 229, 180},
	{"PEARL", "pearl", 240, 234, 214},
	{"PERIWINKLE", "periwinkle", 204, 204, 255},
	{"PINE_GREEN", "pine green", 1, 121, 111},
	{"PINK", "pink", 255, 192, 203},
	{"PLUM", "plum", 102, 0, 102},
	{"PUCE", "puce", 204, 136, 153},
	{"PUMPKIN", "pumpkin", 255, 117, 24},
	{"PURPLE", "purple", 102, 0, 153},
	{"RAW_UMBER", "raw umber", 115, 74, 18},
	{"RED", "red", 255, 0, 0},
	{"RED_PURPLE", "red-purple", 178, 0, 75},
	{"ROSE", "rose", 244, 194, 194},
	{"RUSSET", "russet", 117, 90, 87},
	{"RUST", "rust", 183, 65, 14},
	{"SAFFRON", "saffron", 244, 196, 48},
	{"SCARLET", "scarlet", 255, 36, 0},
	{"SEA_GREEN", "sea green", 46, 139, 87},
	{"SEPIA", "sepia", 112, 66, 20},
	{"SILVER", "silver", 192, 192, 192},
	{"SKY_BLUE", "sky blue", 135, 206, 235},
	{"SLATE_GRAY", "slate gray", 112, 128, 144},
	{"SPRING_GREEN", "spring green", 0, 255, 127},
	{"TAN", "tan", 210, 180, 140},
	{"TAUPE_DARK", "dark taupe", 72, 60, 50},
	{"TAUPE_GRAY", "taupe gray", 139, 133, 137},
	{"TAUPE_MEDIUM", "taupe", 103, 76, 71},
	{"TAUPE_PURPLE", "purple taupe", 80, 64, 77},
	{"TAUPE_PALE", "pale taupe", 188, 152, 126},
	{"TAUPE_ROSE", "rose taupe", 144, 93, 93},
	{"TAUPE_SANDY", "sandy taupe", 150, 113, 23},
	{"TEAL", "teal", 0, 128, 128},
	{"TURQUOISE", "turquoise", 48, 213, 200},
	{"VERMILION", "vermilion", 227, 66, 52},
	{"VIOLET", "violet", 139, 0, 255},
	{"WHITE", "white", 255, 255, 255},
	{"YELLOW", "yellow", 255, 255, 0},
	{"YELLOW_GREEN", "yellow-green", 154, 205, 50},
}

// dfColorMap é um mapa indexado pelo token para lookup rápido.
var dfColorMap map[string]DFColor

func init() {
	dfColorMap = make(map[string]DFColor, len(DFColorList))
	for _, c := range DFColorList {
		dfColorMap[c.Token] = c
	}
}

// GetDFColor retorna a cor RGB para um token de cor do DF.
// Ex: GetDFColor("AMBER") → {255, 191, 0}
func GetDFColor(token string) (uint8, uint8, uint8, bool) {
	if c, ok := dfColorMap[token]; ok {
		return c.R, c.G, c.B, true
	}
	return 128, 128, 128, false
}

// FindNearestDFColor encontra a cor DF mais próxima de um RGB dado.
// Usa distância euclidiana no espaço RGB.
func FindNearestDFColor(r, g, b uint8) string {
	bestToken := "GRAY"
	bestDist := int32(999999)
	for _, c := range DFColorList {
		dr := int32(r) - int32(c.R)
		dg := int32(g) - int32(c.G)
		db := int32(b) - int32(c.B)
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestToken = c.Token
		}
	}
	return bestToken
}
