package fdist

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/krokomoko/fuzzy"
)

type Class struct {
	Values      []float64 `json:"values"` // список, по которому идёт сравнение
	__valuesSum []float64 // список, существующий только для обучения
	Ymu         []float64 `json:"ymu"`   // список средних значений степеней принадлежности (распределение)
	Count       float64   `json:"count"` // количество строк из обучающей выборки, подходящих классу
}

type Distribution struct {
	Classes    []Class           `json:"classes"`
	Parameters []fuzzy.Parameter `json:"parameters"`
	cInd       int               // индекс зависимого значения (параметра)
}

func NewDistribution(data [][]float64, wordsCount, yWordsCount int, classDistance float64) Distribution {
	var dataLen = len(data)

	if dataLen == 0 {
		panic("Data is empty")
	}

	var (
		distribution = Distribution{
			Classes:    []Class{},
			Parameters: make([]fuzzy.Parameter, len(data[0])),
			cInd:       len(data[0]) - 1,
		}
		class      *Class
		values     = make([]float64, dataLen)
		classified = make(map[int]bool)
		ok         bool
		rowLen     = len(data[0]) - 1
	)

	for i := range distribution.Parameters[:rowLen] {
		for j, row := range data {
			values[j] = row[i]
		}
		distribution.Parameters[i] = fuzzy.NewParameter(values, wordsCount)
	}
	for j, row := range data {
		values[j] = row[rowLen]
	}
	distribution.Parameters[rowLen] = fuzzy.NewParameter(values, yWordsCount)

	for i := range data {
		if _, ok = classified[i]; ok {
			continue
		}

		class = distribution.addClass(data[i])

		for j := i + 1; j < dataLen; j++ {
			if distribution.distance(class, data[j][:rowLen]) <= classDistance {
				distribution.addValue(class, data[j][:rowLen], data[j][rowLen])
				classified[j] = true
			}
		}
	}

	for ci := range distribution.Classes {
		for yi := range distribution.Classes[ci].Ymu {
			distribution.Classes[ci].Ymu[yi] /= distribution.Classes[ci].Count
		}

		for vi := range distribution.Classes[ci].__valuesSum {
			distribution.Classes[ci].Values[vi] =
				distribution.Classes[ci].__valuesSum[vi] / distribution.Classes[ci].Count
		}
		distribution.Classes[ci].__valuesSum = []float64{}
	}

	return distribution
}

func (dist *Distribution) addClass(data []float64) *Class {
	var (
		dataLen = len(data) - 1
		mu      float64
	)

	class := Class{
		Values:      make([]float64, dataLen),
		__valuesSum: make([]float64, dataLen),
		Ymu:         make([]float64, len(dist.Parameters[dist.cInd].Words)),
		Count:       1,
	}

	copy(class.Values, data)
	copy(class.__valuesSum, data)

	for wi, word := range dist.Parameters[dist.cInd].Words {
		mu, _ = word.Mu(data[dist.cInd])
		class.Ymu[wi] += mu
	}

	dist.Classes = append(dist.Classes, class)

	return &dist.Classes[len(dist.Classes)-1]
}

func (dist *Distribution) addValue(class *Class, values []float64, value float64) {
	var mu float64
	for wi, word := range dist.Parameters[dist.cInd].Words {
		mu, _ = word.Mu(value)
		class.Ymu[wi] += mu
	}

	for vi := range class.__valuesSum {
		class.__valuesSum[vi] += values[vi]
	}

	class.Count += 1
}

func (dist *Distribution) distance(class *Class, values []float64) float64 {
	var sum, tSum, wCount, mu1, mu2 float64

	for pi, parameter := range dist.Parameters[:dist.cInd] {

		// возможно в будущем у разных параметров будет
		// разное количество слов
		wCount = float64(len(parameter.Words))

		sum = 0

		for _, word := range parameter.Words {
			mu1, _ = word.Mu(values[pi])
			mu2, _ = word.Mu(class.Values[pi])
			sum += math.Abs(mu1 - mu2)
		}

		tSum += sum / wCount
	}

	return tSum / float64(len(dist.Parameters))
}

func (dist *Distribution) GetClass(data []float64, classDistance float64) (*Class, error) {
	var (
		distance float64
		min          = math.MaxFloat64
		minInd   int = -1
	)

	for classInd, class := range dist.Classes {
		distance = dist.distance(&class, data)
		if distance <= classDistance && distance < min {
			min = distance
			minInd = classInd
		}
	}

	if minInd == -1 {
		return nil, fmt.Errorf("No matching data class")
	}

	return &dist.Classes[minInd], nil
}

func (dist *Distribution) Mean(class *Class) (float64, error) {
	if class == nil {
		return 0.0, fmt.Errorf("Class pointer is nil")
	}

	return dist.Parameters[dist.cInd].Value(class.Ymu)
}

func __min(a, b float64) float64 {
	if a > b {
		return b
	}
	return a
}

func __max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// только для версии fuzzy до 0642ac98b0f620b18b62ebc8c39c233d9555fd86
func (dist *Distribution) ProbFromTo(class *Class, from, to float64) (p float64) {
	var (
		_from, _to float64
		_mx, _d    float64

		d           = to - from
		lastWordInd = len(class.Ymu) - 1
	)

	if d <= 0 {
		panic("from >= to")
	}

	for wordInd, word := range dist.Parameters[dist.cInd].Words {
		if word.Min >= to {
			break
		}
		if word.Max <= from {
			continue
		}

		// вычисление по текущему
		// + вычисление по пересечению со следующим

		// вероятность значений текущего слова
		_from = __max(word.Min, from)
		if wordInd > 0 {
			// prev
			_from = __max(dist.Parameters[dist.cInd].Words[wordInd-1].Max, _from)
		}
		_to = __min(word.Max, to)
		if wordInd < lastWordInd {
			// next
			_to = __min(_to, dist.Parameters[dist.cInd].Words[wordInd+1].Min)
		}
		if _to > _from {
			//p += class.Ymu[wordInd] * (_to - _from) / (word.Max - word.Min)
			p += class.Ymu[wordInd]
		}

		// вероятность персечения значений текущего слова и следующего
		if wordInd < lastWordInd {
			// next
			_from = __max(dist.Parameters[dist.cInd].Words[wordInd+1].Min, from)
			_to = __min(word.Max, to)

			if _to > _from {
				_mx = __max(class.Ymu[wordInd], class.Ymu[wordInd+1])
				if _mx == class.Ymu[wordInd] {
					_d =
						dist.Parameters[dist.cInd].Words[wordInd].Max -
							dist.Parameters[dist.cInd].Words[wordInd].Min
				} else {
					_d =
						dist.Parameters[dist.cInd].Words[wordInd+1].Max -
							dist.Parameters[dist.cInd].Words[wordInd+1].Min
				}
				p += _mx * (_to - _from) / _d
				//p += __max(class.Ymu[wordInd], class.Ymu[wordInd+1]) * (_to - _from) / d
				//p += 0.5 * (class.Ymu[wordInd] + class.Ymu[wordInd+1]) * (_to - _from) / d
			}
		}
	}

	return
}

func (dist *Distribution) Save(filename string) (err error) {
	var output []byte

	output, err = json.Marshal(dist)
	if err != nil {
		err = fmt.Errorf("Ошибка сериализация данных: %w", err)
		return
	}

	err = os.WriteFile(filename, output, 0666)
	if err != nil {
		err = fmt.Errorf("Ошибка записи данных в файл: %w", err)
	}

	return
}

func Load(filename string) (dist *Distribution, err error) {
	var content []byte

	content, err = os.ReadFile(filename)
	if err != nil {
		err = fmt.Errorf("Ошибка чтения файла: %w", err)
		return
	}

	dist = &Distribution{}

	err = json.Unmarshal(content, dist)
	if err != nil {
		err = fmt.Errorf("Ошибка десериализации данных: %w", err)
		return
	}

	dist.cInd = len(dist.Parameters) - 1

	return
}
