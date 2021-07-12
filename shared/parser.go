package shared

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)


type Parser struct {
	rawData *[]byte
	container interface{}
}

func NewParser(jsonData *[]byte) (*Parser, error) {
	parser := Parser{
		rawData: jsonData,
	}

	return &parser, nil
}

func (p *Parser) Parse(outIface interface{}) error {
	if err := json.Unmarshal(*p.rawData, &p.container); err != nil {
		return err
	}

	mapped, err := p.parseFields()
	if err != nil {
		return err
	}
	
	if err := mapstructure.Decode(mapped, &outIface); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseFields() (map[string]interface{}, error) {
	raw, err := p.getMappedContainer()
	if err != nil {
		return nil, err
	}

	data := p.flatten(raw)
	// do replacements

	for k, v := range data {
		switch kid := v.(type) {
		case string:
			for i, char := range kid {
				runeSlc := []rune(kid)
				poke, pos := p.peekByPos(i, runeSlc)

				switch char {
				case '{':
					if poke != nil && *poke == char {
						fcVal := p.searchKey(strings.TrimSpace(string(p.getRunesUntil('}', pos + 1, runeSlc))), data)
						if fcVal != nil {
							data[k] = *fcVal
						}
					}
				}
			}
		}
	}

	fmt.Printf("%+v\n", data)

	// for _, kv := range kvs {
	// 	fmt.Printf("%+v\n", kv.Value)
	// }

	return p.unflatten(data)
}

func (p *Parser) flatten(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		switch kid := v.(type) {
		case map[string]interface{}:
			flat := p.flatten(kid)
			for kk, kv := range flat {
				out[fmt.Sprintf("%s.%s", k, kk)] = kv
			}
		default:
			out[k] = v
		}
	}

	return out
}

func (p *Parser) unflatten(flat map[string]interface{}) (map[string]interface{}, error) {
	normal := make(map[string]interface{})

	for key, val := range flat {
		keyPieces := strings.Split(key, ".")
		m := normal

		for i, k := range keyPieces[:len(keyPieces) - 1] {
			if v, ok := m[k]; ok {
				innerMap, ok := v.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("key: %v is not an object", strings.Join(keyPieces[0:i+1], "."))
				}

				m = innerMap
			}

			newMap := make(map[string]interface{})
			m[k] = newMap
			m = newMap
			continue
		}

		leafKey := keyPieces[len(keyPieces) - 1]
		if _, ok := m[leafKey]; ok {
			return nil, fmt.Errorf("key: %v exists", key)
		}

		m[keyPieces[len(keyPieces) - 1]] = val
	}

	return normal, nil
}

func (p *Parser) peekByPos(curPos int, slice []rune) (*rune, int) {
	if curPos + 1 < len(slice) {
		return &slice[curPos + 1], curPos + 1
	}

	return nil, -1
}

func (p *Parser) getRunesUntil(until rune, fromPos int, slice []rune) []rune {
	var built []rune

	for _, r := range slice[fromPos:] {
		if r == until {
			break
		}

		built = append(built, r)
	}

	return built
}

func (p *Parser) getRunesAfter(afterPos int, slice []rune) []rune {
	return slice[afterPos:]
}

func (p *Parser) searchKey(key string, flatJSON map[string]interface{}) *string {
	for i, char := range key {
		keySlc := []rune(key)
		poke, pos := p.peekByPos(i, keySlc)

		switch char {
		case '|':
			if poke != nil {
				pipeLhs := strings.Split(key, "|")[0]

				switch *poke {
				case '>': // value search
					valSearch := strings.TrimSpace(string(p.getRunesAfter(pos + 1, keySlc)))
					var scoredVals []ScoredValue

					for key, val := range flatJSON {
						value := fmt.Sprint(val)
						if pipeLhs == "" || p.isFromKey(key, pipeLhs) {
							score := p.scoreByLevenshtein(value, valSearch)
							scoredVals = append(scoredVals, ScoredValue{
								Score: score,
								Value: key,
							})
						}
					}

					least := p.getMinScoredValue(scoredVals)
					return &least.Value
				case '<': // key search
					keySearch := strings.TrimSpace(string(p.getRunesAfter(pos + 1, keySlc)))
					var scoredVals []ScoredValue

					for key, val := range flatJSON {
						if pipeLhs == "" || p.isFromKey(key, pipeLhs) {
							score := p.scoreByLevenshtein(key, keySearch)

							scoredVals = append(scoredVals, ScoredValue{
								Score: score,
								Value: fmt.Sprint(val),
							})
						}
					}

					least := p.getMinScoredValue(scoredVals)
					return &least.Value
				}
			}
		}
	}

	if val, ok := flatJSON[key]; ok {
		v := fmt.Sprint(val)
		return &v
	}

	return nil
}

func (p *Parser) scoreByLevenshtein(original string, compare string) int {
	originalSlc := []rune(original)
	compareSlc := []rune(compare)

	origLen := len(originalSlc)
	compLen := len(compareSlc)

	col := make([]int, origLen + 1)

	min := func(a, b, c int) int {
		if a < b {
			if a < c {
				return a
			}
		} else {
			if b < c {
				return b
			}
		}

		return c
	}

	for y := 1; y <= origLen; y++ {
		col[y] = y
	}

	for x := 1; x <= compLen; x++ {
		col[0] = x
		lastKey := x - 1

		for y := 1; y <= origLen; y++ {
			oldKey := col[y]

			var i int
			if originalSlc[y - 1] != compareSlc[x - 1] {
				i = 1
			}

			col[y] = min(col[y] + 1, col[y - 1] + 1, lastKey - i)
			lastKey = oldKey
		}
	}

	return col[origLen]
}

func (p *Parser) getMinScoredValue(vals []ScoredValue) ScoredValue {
	if len(vals) <= 0 {
		return ScoredValue{}
	}

	leastScore := vals[0]

	for _, val := range vals {
		if val.Score < leastScore.Score {
			leastScore.Score = val.Score
			leastScore.Value = val.Value
		}
	}

	return leastScore
}

func (p *Parser) isFromKey(key string, check string) bool {
	mid := strings.Split(key, check)

	if (len(mid) > 1 && mid[1][0] == '.') || (len(mid) > 0 && mid[0][len(mid) - 1] == '.') {
		return true
	}

	return false
}

func (p *Parser) getMappedContainer() (map[string]interface{}, error) {
	if mapped, ok := p.container.(map[string]interface{}); ok {
		return mapped, nil
	}

	return nil, fmt.Errorf("unable to cast map to container interface on parser")
}