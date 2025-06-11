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
	Classes    List              `json:"classes"`
	Parameters []fuzzy.Parameter `json:"parameters"`
	cInd       int               // индекс зависимого значения (параметра)
}

func NewDistribution(data [][]float64, wordsCount, yWordsCount int, classDistance float32) Distribution {
	var dataLen = len(data)

	if dataLen == 0 {
		panic("Data is empty")
	}

	var (
		distribution = Distribution{
			Classes:    List{},
			Parameters: make([]fuzzy.Parameter, len(data[0])),
			cInd:       len(data[0]) - 1,
		}
		values = make([]float64, dataLen)
		rowLen = len(data[0]) - 1
	)

	for i := range rowLen + 1 {
		for j, row := range data {
			values[j] = row[i]
		}
		if i == rowLen {
			wordsCount = yWordsCount
		}
		distribution.Parameters[i] = fuzzy.NewParameter(values, wordsCount)
	}

	// TODO: обойти повторное вычисление расстояний
	// TODO: способы оптимизации??????
	var (
		class     *Class
		distance  float32
		ok        bool
		distances = make(map[int]map[int]float32)
	)
	for i := range data {
		class = distribution.addClass(data[i])

		for j := range data {
			if i == j {
				continue
			}
			if distance, ok = distances[i][j]; ok {
				if distance <= classDistance {
					distribution.addValue(class, data[j][:rowLen], data[j][rowLen])
				}
				continue
			}
			if distance = distribution.distance(class.Values, data[j][:rowLen]); distance <= classDistance {
				distribution.addValue(class, data[j][:rowLen], data[j][rowLen])
				if _, ok = distances[j]; !ok {
					distances[j] = make(map[int]float32)
				}
				distances[j][i] = distance
			}
		}
	}

	// вычисление средних значений для определения
	// распределения искомой величины и класса,
	// соответствующего предоставленным данным
	classNode := distribution.Classes.start
	for classNode != nil {
		count := classNode.value.Count
		for yi := range classNode.value.Ymu {
			classNode.value.Ymu[yi] /= count
		}
		for vi := range classNode.value.__valuesSum {
			classNode.value.Values[vi] =
				classNode.value.__valuesSum[vi] / count
		}
		clear(classNode.value.__valuesSum)
		classNode = classNode.next
	}

	// мерждинг классов
	for ci := distribution.Classes.start; ci != nil; ci = ci.next {
		for cj := ci.next; cj != nil; cj = cj.next {
			if distribution.distance(ci.value.Values, cj.value.Values) <= classDistance {
				distribution.mergeClasses(ci.value, cj.value)
				cj.prev.next = cj.next
				if cj.next != nil {
					cj.next.prev = cj.prev
				}
			}
		}
	}

	return distribution
}

func (dist *Distribution) mergeClasses(c1, c2 *Class) {
	for i := range c1.Values {
		c1.Values[i] = (c1.Values[i] + c2.Values[i]) / 2.0
	}

	for i := range c1.Ymu {
		c1.Ymu[i] = (c1.Ymu[i] + c2.Ymu[i]) / 2.0
	}
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

	dist.Classes.add(&class)

	return &class
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

func (dist *Distribution) distance(v1, v2 []float64) float32 {
	var sum, tSum, wCount, mu1, mu2 float64

	for pi, parameter := range dist.Parameters[:dist.cInd] {

		// возможно в будущем у разных параметров будет
		// разное количество слов
		wCount = float64(len(parameter.Words))

		sum = 0

		for _, word := range parameter.Words {
			mu1, _ = word.Mu(v1[pi])
			mu2, _ = word.Mu(v2[pi])
			sum += math.Abs(mu1 - mu2)
		}

		tSum += sum / wCount
	}

	return float32(tSum) / float32(len(dist.Parameters))
}

func (dist *Distribution) GetClass(data []float64, classDistance float32) (*Class, error) {
	var (
		result   *Class
		distance float32
		min      float32 = math.MaxFloat32
	)

	for node := dist.Classes.start; node != nil; node = node.next {
		distance = dist.distance(node.value.Values, data)
		if distance <= classDistance && distance < min {
			min = distance
			result = node.value
		}
	}

	if result == nil {
		return nil, fmt.Errorf("нет подходящего класса")
	}

	return result, nil
}

func (dist *Distribution) Mean(class *Class) (float64, error) {
	if class == nil {
		return 0.0, fmt.Errorf("Class pointer is nil")
	}

	return dist.Parameters[dist.cInd].Value(class.Ymu)
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
		_from = max(word.Min, from)
		if wordInd > 0 {
			// prev
			_from = max(dist.Parameters[dist.cInd].Words[wordInd-1].Max, _from)
		}
		_to = min(word.Max, to)
		if wordInd < lastWordInd {
			// next
			_to = min(_to, dist.Parameters[dist.cInd].Words[wordInd+1].Min)
		}
		if _to > _from {
			//p += class.Ymu[wordInd] * (_to - _from) / (word.Max - word.Min)
			p += class.Ymu[wordInd]
		}

		// вероятность персечения значений текущего слова и следующего
		if wordInd < lastWordInd {
			// next
			_from = max(dist.Parameters[dist.cInd].Words[wordInd+1].Min, from)
			_to = min(word.Max, to)

			if _to > _from {
				_mx = max(class.Ymu[wordInd], class.Ymu[wordInd+1])
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
		err = fmt.Errorf("ошибка сериализация данных: %w", err)
		return
	}

	err = os.WriteFile(filename, output, 0666)
	if err != nil {
		err = fmt.Errorf("ошибка записи данных в файл: %w", err)
	}

	return
}

func Load(filename string) (dist *Distribution, err error) {
	var content []byte

	content, err = os.ReadFile(filename)
	if err != nil {
		err = fmt.Errorf("ошибка чтения файла: %w", err)
		return
	}

	dist = &Distribution{}

	err = json.Unmarshal(content, dist)
	if err != nil {
		err = fmt.Errorf("ошибка десериализации данных: %w", err)
		return
	}

	dist.cInd = len(dist.Parameters) - 1

	return
}
