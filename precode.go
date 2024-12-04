package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и
// отправляет их в канал ch. При этом после записи в канал для каждого числа
// вызывается функция fn. Она служит для подсчёта количества и суммы
// сгенерированных чисел.
// 1. Функция Generator
// Функция должна генерировать числа и записывать их в канал ch. Для каждого числа должна быть вызвана функция fn(). Используйте такой алгоритм генерации:
// - N(0) = 1;
// - N(i) = N(i-1) + 1
// 'Generator() прекращает работу, когда поступил сигнал об отмене контекста ctx.
// Используйте бесконечный цикл for и конструкцию select с проверкой на <-ctx.Done(). Перед выходом из функции закройте канал ch.
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	// 1. Функция Generator
	var current int64 = 1
	defer close(ch)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ch <- current
			fn(current)
			current++
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.
// Эта функция обрабатывает числа. Она должна читать числа из канала in до тех пор, пока он не закроется.
// Полученное число нужно записать в канал out и сделать паузу на 1 миллисекунду. Если канал in закрылся, то нужно закрыть канал out и завершить работу функции.
// Можно использовать бесконечный цикл и оператор v, ok := <-in, который позволяет отследить закрытие канала.
func Worker(in <-chan int64, out chan<- int64) {
	// 2. Функция Worker
	for {
		v, ok := <-in
		if !ok {
			close(out)
			return
		}

		out <- v
		time.Sleep(time.Millisecond)
	}
}

func main() {
	chIn := make(chan int64)

	// 3. Создание контекста
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// для проверки будем считать количество и сумму отправленных чисел
	//было
	//var inputSum int64   // сумма сгенерированных чисел
	//var inputCount int64 // количество сгенерированных чисел
	//стало (для задания 6)
	var inputSum atomic.Int64
	var inputCount atomic.Int64

	// генерируем числа, считая параллельно их количество и сумму
	go Generator(ctx, chIn, func(i int64) {
		//	inputSum += i
		//	inputCount++
		inputSum.Add(i)
		inputCount.Add(1)
	})

	const NumOut = 5 // количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		// создаём каналы и для каждого из них вызываем горутину Worker
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// 4. Собираем числа из каналов outs
	// В этом фрагменте нужно пройтись по всем каналам из слайса outs и для каждого из них запустить горутину с анонимной функцией.
	// Горутина должна читать данные из переданного ей канала, до тех пор, пока он не закроется.
	// Чтобы дождаться завершения всех этих горутин, нужно воспользоваться механизмом WaitGroup.
	// Горутина должна подсчитывать обработанные числа (увеличивать соответствующий счётчик в слайсе amounts) передавать значения в канал chOut.
	// Перед вызовом горутины не забудьте вызвать wg.Add(1), а при её завершении — wg.Done().
	// Используйте анонимную функцию с двумя параметрами go func(in <-chan int64, i int64){}, где in  — очередной канал из outs, а i — его индекс.
	// Не забывайте про увеличение счётчика amounts[i]++.

	for i := 0; i < NumOut; i++ {
		wg.Add(1)
		go func(in <-chan int64, i int64) {
			defer wg.Done()
			for num := range in {
				chOut <- num
				amounts[i]++
			}
		}(outs[i], int64(i))
	}

	go func() {
		// ждём завершения работы всех горутин для outs
		wg.Wait()
		// закрываем результирующий канал
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	// Осталось прочитать все данные из канала chOut. При этом нужно подсчитать в переменной count общее количество чисел, а в переменной sum — сумму всех чисел.
	// Воспользуйтесь конструкцией for ... range, которая будет читать числа из канала до его закрытия.
	for num := range chOut {
		sum += num
		count++
	}

	fmt.Println("Количество чисел", inputCount.Load(), count)
	fmt.Println("Сумма чисел", inputSum.Load(), sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
	if inputSum.Load() != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum.Load(), sum)
	}
	if inputCount.Load() != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount.Load(), count)
	}
	for _, v := range amounts {
		inputCount.Add(-v)
	}
	if inputCount.Load() != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
