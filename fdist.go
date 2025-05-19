package fdist

import (
	"fmt"
	"math"

	"github.com/krokomoko/fuzzy"
)

type Class struct {
	values      []float64 // список, по которому идёт сравнение
	__valuesSum []float64 // список, существующий только для обучения
	ymu         []float64 // список средних значений степеней принадлежности (распределение)
	count       float64   // количество строк из обучающей выборки, подходящих классу
}

type Distribution struct {
	classes    []Class
	cInd       int // индекс зависимого значения (параметра)
	Parameters []fuzzy.Parameter
}

func NewDistribution(data [][]float64, wordsCount, yWordsCount int, classDistance float64) Distribution {
	var dataLen = len(data)

	if dataLen == 0 {
		panic("Data is empty")
	}

	var (
		distribution = Distribution{
			classes:    []Class{},
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

	for ci := range distribution.classes {
		for yi := range distribution.classes[ci].ymu {
			distribution.classes[ci].ymu[yi] /= distribution.classes[ci].count
		}

		for vi := range distribution.classes[ci].__valuesSum {
			distribution.classes[ci].values[vi] =
				distribution.classes[ci].__valuesSum[vi] / distribution.classes[ci].count
		}
		distribution.classes[ci].__valuesSum = []float64{}
	}

	return distribution
}

func (dist *Distribution) addClass(data []float64) *Class {
	var (
		dataLen = len(data) - 1
		mu      float64
	)

	class := Class{
		values:      make([]float64, dataLen),
		__valuesSum: make([]float64, dataLen),
		ymu:         make([]float64, len(dist.Parameters[dist.cInd].Words)),
		count:       1,
	}

	copy(class.values, data)
	copy(class.__valuesSum, data)

	for wi, word := range dist.Parameters[dist.cInd].Words {
		mu, _ = word.Mu(data[dist.cInd])
		class.ymu[wi] += mu
	}

	dist.classes = append(dist.classes, class)

	return &dist.classes[len(dist.classes)-1]
}

func (dist *Distribution) addValue(class *Class, values []float64, value float64) {
	var mu float64
	for wi, word := range dist.Parameters[dist.cInd].Words {
		mu, _ = word.Mu(value)
		class.ymu[wi] += mu
	}

	for vi := range class.__valuesSum {
		class.__valuesSum[vi] += values[vi]
	}

	class.count += 1
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
			mu2, _ = word.Mu(class.values[pi])
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

	for classInd, class := range dist.classes {
		distance = dist.distance(&class, data)
		if distance <= classDistance && distance < min {
			min = distance
			minInd = classInd
		}
	}

	if minInd == -1 {
		return nil, fmt.Errorf("No matching data class")
	}

	return &dist.classes[minInd], nil
}

func (dist *Distribution) Mean(class *Class) (float64, error) {
	if class == nil {
		return 0.0, fmt.Errorf("Class pointer is nil")
	}

	return dist.Parameters[dist.cInd].Value(class.ymu)
}
