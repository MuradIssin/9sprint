package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	defer close(ch)
	var i int64 = 1
	for {
		select {
		case <-ctx.Done():
			return
		case ch <- i:
			fn(i)
			i++
		}
	}
}

func Worker(in <-chan int64, out chan<- int64) {
	defer close(out)
	for num := range in {
		out <- num
		time.Sleep(1 * time.Millisecond)
	}
}

func main() {
	chIn := make(chan int64)

	// Создание контекста
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel() // Освобождаем ресурсы контекста по завершении

	// Для проверки будем считать количество и сумму отправленных чисел
	var inputSum int64   // Сумма сгенерированных чисел
	var inputCount int64 // Количество сгенерированных чисел

	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)
		atomic.AddInt64(&inputCount, 1)
	})

	const NumOut = 5 // Количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i]) // Запускаем горутину Worker для каждого канала
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// Собираем числа из каналов outs
	for i := 0; i < NumOut; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for num := range outs[i] {
				chOut <- num
				atomic.AddInt64(&amounts[i], 1) // Потокобезопасно увеличиваем статистику
			}
		}(i)

	}

	go func() {

		wg.Wait()

		close(chOut)
	}()

	var count int64 // Количество чисел результирующего канала
	var sum int64   // Сумма чисел результирующего канала

	for num := range chOut {
		count++
		sum += num
	}

	fmt.Println("Количество чисел", atomic.LoadInt64(&inputCount), count)
	fmt.Println("Сумма чисел", atomic.LoadInt64(&inputSum), sum)
	fmt.Println("Разбивка по каналам", amounts)

	// Проверка результатов
	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}
	for _, v := range amounts {
		atomic.AddInt64(&inputCount, -v)
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
