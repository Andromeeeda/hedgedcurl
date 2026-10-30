package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {

	//Обработка флагов и url

	var timeout int
	var help bool
	flag.IntVar(&timeout, "t", 15, "таймаут в секундах")
	flag.IntVar(&timeout, "timeout", 15, "таймаут в секундах")

	flag.BoolVar(&help, "h", false, "показать справку")
	flag.BoolVar(&help, "help", false, "показать справку")

	flag.Parse()

	urls := flag.Args()

	if help {
		printHelp()
		os.Exit(0)
	}

	if len(urls) == 0 {
		fmt.Println("Не задан URL")
		os.Exit(1)
	}

	fmt.Println("timeout:", timeout)
	fmt.Println("Urls:", urls)

	//создание контекста с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	firstResponse := make(chan *http.Response, 1)

	errors := make(chan error, len(urls))

	var wg sync.WaitGroup

	// Запускаем горутину на каждый URL
	for _, url := range urls {
		wg.Add(1)

		go func(requestURL string) {
			defer wg.Done()

			//  запрос с контекстом (для возможности отмены)
			req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
			if err != nil {
				select {
				case errors <- fmt.Errorf("URL %s: ошибка создания запроса: %v", requestURL, err):
				default:
				}
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				select {
				case errors <- fmt.Errorf("URL %s: ошибка запроса: %v", requestURL, err):
				default:
				}
				return
			}

			select {
			case firstResponse <- resp:

			default:

				resp.Body.Close()
			}
		}(url)
	}

	// Канал, который закроется, когда все горутины завершатся
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case resp := <-firstResponse:
		printResponse(resp)
		resp.Body.Close()

	case <-ctx.Done():
		fmt.Println("Ошибка: превышен таймаут")
		os.Exit(228)

	case <-done:
		fmt.Println("Ошибка: все запросы завершились ошибкой")
		os.Exit(1)
	}
}

// printHelp выводит справку по использованию программы
func printHelp() {
	fmt.Println("hedgedcurl - утилита для хеджированных HTTP запросов")
	fmt.Println("\nИспользование:")
	fmt.Println("  hedgedcurl [флаги] URL1 URL2 URL3 ...")
	fmt.Println("\nФлаги:")
	fmt.Println("  -t SECONDS     таймаут в секундах (по умолчанию 15)")
	fmt.Println("  -h, --help     показать эту справку")
	fmt.Println("\nПример:")
	fmt.Println("  hedgedcurl https://example1.ru https://example2.ru")
}

// printResponse выводит статус, заголовки и тело ответа
func printResponse(resp *http.Response) {
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	fmt.Println()

	_, err := io.Copy(os.Stdout, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при чтении тела ответа: %v\n", err)
	}

	fmt.Println()
}
