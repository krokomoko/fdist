package fdist

import (
	"math"

	"github.com/krokomoko/fuzzy"
)

type Distribution struct {
	Ymu        []float64         `json:"ymu"`   // список средних значений степеней принадлежности (распределение)
	Count      float64           `json:"count"` // количество строк из обучающей выборки, подходящих классу
	Parameters []fuzzy.Parameter `json:"parameters"`
	cind       int               // индекс зависимого значения (параметра)
}

type Config struct {
	K, Words, YWords int
	Distance         float64
}

func GetDistribution(conf *Config, x []float64, samples [][]float64) *Distribution {
	var ln = len(samples)

	if ln == 0 {
		return nil
	}

	var (
		dist = Distribution{
			Parameters: make([]fuzzy.Parameter, len(samples[0])),
			cind:       len(samples[0]) - 1,
		}
		values = make([]float64, ln)
	)

	for i := range dist.cind + 1 {
		for j, row := range samples {
			values[j] = row[i]
		}
		if i == dist.cind {
			dist.Parameters[i] = fuzzy.NewParameter(values, conf.YWords)
		} else {
			dist.Parameters[i] = fuzzy.NewParameter(values, conf.Words)
		}
	}

	dist.Ymu = make([]float64, len(dist.Parameters[dist.cind].Words))

	heap := NewHeap(conf.K)

	var distance float64
	for i := range samples {
		distance = dist.Distance(x, samples[i][:dist.cind])
		if distance <= conf.Distance {
			heap.Add(i, distance)
		}
	}

	var mu float64
	for _, el := range *heap.Heap {
		for wi, word := range dist.Parameters[dist.cind].Words {
			mu, _ = word.Mu(samples[el.i][dist.cind])
			dist.Ymu[wi] += mu
		}
	}

	var depth = float64(heap.Heap.Len())
	for wi := range dist.Parameters[dist.cind].Words {
		dist.Ymu[wi] /= depth
	}

	return &dist
}

func (dist *Distribution) Distance(v1, v2 []float64) float64 {
	var sum, tSum, wCount, mu1, mu2 float64

	for pi, parameter := range dist.Parameters[:dist.cind] {

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

	return tSum / float64(len(dist.Parameters))
}

func (dist *Distribution) Mean() (float64, error) {
	return dist.Parameters[dist.cind].Value(dist.Ymu)
}

func (dist *Distribution) ProbFromTo(from, to float64) (p float64) {
	var (
		_from, _to float64
		_mx, _d    float64

		d           = to - from
		lastWordInd = len(dist.Ymu) - 1
	)

	if d <= 0 {
		panic("from >= to")
	}

	for wordInd, word := range dist.Parameters[dist.cind].Words {
		if word.Min >= to {
			break
		}
		if word.Max <= from {
			continue
		}

		// вероятность значений текущего слова
		_from = max(word.Min, from)
		if wordInd > 0 {
			// prev
			_from = max(dist.Parameters[dist.cind].Words[wordInd-1].Max, _from)
		}
		_to = min(word.Max, to)
		if wordInd < lastWordInd {
			// next
			_to = min(_to, dist.Parameters[dist.cind].Words[wordInd+1].Min)
		}
		if _to > _from {
			//p += class.Ymu[wordInd] * (_to - _from) / (word.Max - word.Min)
			p += dist.Ymu[wordInd]
		}

		// вероятность персечения значений текущего слова и следующего
		if wordInd < lastWordInd {
			// next
			_from = max(dist.Parameters[dist.cind].Words[wordInd+1].Min, from)
			_to = min(word.Max, to)

			if _to > _from {
				_mx = max(dist.Ymu[wordInd], dist.Ymu[wordInd+1])
				if _mx == dist.Ymu[wordInd] {
					_d =
						dist.Parameters[dist.cind].Words[wordInd].Max -
							dist.Parameters[dist.cind].Words[wordInd].Min
				} else {
					_d =
						dist.Parameters[dist.cind].Words[wordInd+1].Max -
							dist.Parameters[dist.cind].Words[wordInd+1].Min
				}
				p += _mx * (_to - _from) / _d
				//p += __max(class.Ymu[wordInd], class.Ymu[wordInd+1]) * (_to - _from) / d
				//p += 0.5 * (class.Ymu[wordInd] + class.Ymu[wordInd+1]) * (_to - _from) / d
			}
		}
	}

	return
}
