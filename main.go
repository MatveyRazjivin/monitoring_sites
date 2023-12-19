package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"time"
	"sync"
	"crypto/tls"
	"net/smtp"
	"bytes"
	"encoding/json"
)

var siteResponseTimes = make(map[string][]time.Duration)
var siteErrors = make(map[string]int)
var siteOK = make(map[string]int)

const telegramBotToken = "AAH5mqpNaoe55hWdAAIcbZqFYrwkw554VTQ"
const telegramAPIBaseURL = "https://api.telegram.org/bot"

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func sendTelegramMessage(chatID int64, message string) error {
	apiURL := fmt.Sprintf("%s%s/sendMessage", telegramAPIBaseURL, telegramBotToken)
	requestBody, err := json.Marshal(sendMessageRequest{
		ChatID: chatID,
		Text:   message,
	})
	if err != nil {
		return err
	}

	response, err := http.Post(apiURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}

func email() {
	from := os.Getenv("youremail@gmail.com")
	password := os.Getenv("*****") //your password

	toEmail := os.Getenv("toemail@yandex.ru")
	to := []string{toEmail}

	host := "smtp.gmail.com"
	port := "587"
	address := host + ":" + port

	subject := "Ошибка на сайте\n"
	body := "Ошибка на сайте"
	message := []byte(subject + body)

	auth := smtp.PlainAuth("", from, password, host)

	err := smtp.SendMail(address, auth, from, to, message)

	if err != nil {
		fmt.Println("err:", err)
		return
	}
}

// Функция для сохранения информации о недоступных сайтах в файлы
func saveSiteInfoToFile(site string, statusCode int) {
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	fileName := "errors.txt"

	// Дозапись в txt файл
	txtFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Ошибка при открытии/создании txt файла для сайта %s: %v\n", site, err)
		return
	}
	defer txtFile.Close()

	txtFile.WriteString(fmt.Sprintf("URL: %s\n", site))
	txtFile.WriteString(fmt.Sprintf("Статус ответа: %d\n", statusCode))
	txtFile.WriteString(fmt.Sprintf("Дата и время: %s\n", currentTime))

	// Дозапись в json файл
	jsonFileName := "errors.json"
	jsonFile, err := os.OpenFile(jsonFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Ошибка при открытии/создании json файла для сайта %s: %v\n", site, err)
		return
	}
	defer jsonFile.Close()

	data := map[string]interface{}{
		"url":         site,
		"status_code": statusCode,
		"timestamp":   currentTime,
	}

	encoder := json.NewEncoder(jsonFile)
	err = encoder.Encode(data)
	if err != nil {
		fmt.Printf("Ошибка при записи данных в json файл для сайта %s: %v\n", site, err)
	}
}

func checkSiteAvailability(site string) int {

	startTime := time.Now()

	resp, err := http.Get(site)
	if err != nil {
		fmt.Printf("Сайт %s недоступен.\nОшибка: %v\n\n", site, err)
		siteErrors[site]++

		return -1 // -1 указывает на ошибку
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)
	siteResponseTimes[site] = append(siteResponseTimes[site], responseTime)

	fmt.Printf("Сайт %s ", site)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == 200 {
		fmt.Printf("доступен.\nСтатус: %s.\nВремя отклика: %v\n\n", resp.Status, responseTime)
		siteOK[site]++
		
	} else {
		fmt.Printf("недоступен.\nСтатус: %s\n", resp.Status)
		siteErrors[site]++

		saveSiteInfoToFile(site, resp.StatusCode)

		chatID := int64(1002116763062)
		message := "Ошибка на сайте"

		err := sendTelegramMessage(chatID, message)
		if err != nil {
			fmt.Println("Ошибка при отправке сообщения:", err, "\n")
		} else {
			fmt.Println("Сообщение успешно отправлено!\n")
		}
	}

	return resp.StatusCode
}

func printMinMaxResponseTimesAndErrors() {
	fmt.Println("Наименьшее, наибольшее и среднее время отклика за последний час:")
	for site, responseTimes := range siteResponseTimes {
		if len(responseTimes) > 0 {
			minTime := responseTimes[0]
			maxTime := responseTimes[0]
			totalTime := time.Duration(0)

			for _, rt := range responseTimes {
				if rt < minTime {
					minTime = rt
				}
				if rt > maxTime {
					maxTime = rt
				}
				totalTime += rt
			}

			averageTime := totalTime / time.Duration(len(responseTimes))

			if siteErrors[site] == 0 {
				fmt.Printf("Сайт: %s\nМинимальное время: %v\nМаксимальное время: %v\nСреднее время: %v\n\n", site, minTime, maxTime, averageTime)
			} else if siteErrors[site] == 60 {
				fmt.Printf("Сайт: %s\nНе был доступен в течение времени проверки.\n", site)
			}

			// Вывести количество ошибок для данного сайта
			if errors, ok := siteErrors[site]; ok {
				fmt.Printf("Количество ошибок: %d\n", errors)
			}
			fmt.Println()
		}
	}
}

func makeRequest(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(start).Milliseconds()
	fmt.Printf("Запрос на %s длился %d ms\n", url, duration)
}

func main() {
	filename := "sites.txt"

	var choice int

	for {
		fmt.Println("Введите вариант:")
		fmt.Println("1. Мониторинг состояния сайтов")
		fmt.Println("2. Нагрузочный тест")
		fmt.Println("3. Проверка сертификата безопасности")
		fmt.Println("4. Завершить программу\n")

		fmt.Scanf("%d\n", &choice)

		// Обработка выбора
		switch choice {
		case 1:
			for {
				file, err := os.Open(filename)
				if err != nil {
					fmt.Printf("Ошибка при открытии файла: %v\n", err)
					return
				}
				defer file.Close()
		
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					site := scanner.Text()
					statusCode := checkSiteAvailability(site)
					_ = statusCode
				}
		
				if err := scanner.Err(); err != nil {
					fmt.Printf("Ошибка при чтении файла: %v\n", err)
				}
		
				// Подождать 1 минуту перед следующей проверкой
				time.Sleep(1 * time.Minute)
		
				// Проверка на завершение часа
				if time.Now().Minute()%60 == 0 {
					// Вывести наименьшее, наибольшее и среднее время отклика за час
					printMinMaxResponseTimesAndErrors()
		
					// Очистить данные о времени отклика для следующего часа
					siteResponseTimes = make(map[string][]time.Duration)
					siteErrors = make(map[string]int)
					siteOK = make(map[string]int)
				}
			}
		case 2:
			// URL сайта, который вы хотите протестировать
			var targetURL string
			fmt.Println("Введите URL сайта:")
			fmt.Scanf("%s\n", &targetURL)

			// Количество запросов
			var concurrentRequests int
			fmt.Println("Введите количество запросов:")
			fmt.Scanf("%d\n", &concurrentRequests)

			// Создание WaitGroup для ожидания завершения всех горутин
			var wg sync.WaitGroup

			fmt.Printf("Запуск нагрузочного теста на %s для %d запросов...\n", targetURL, concurrentRequests)

			// Запуск запросов
			for i := 0; i < concurrentRequests; i++ {
				wg.Add(1)
				go makeRequest(targetURL, &wg)
			}

			// Ожидание завершения всех горутин
			wg.Wait()

			fmt.Println("Нагрузочное тестирование завершено\n")
		case 3:
			var url string
			fmt.Println("Введите URL сайта:")
			fmt.Scanf("%s\n", &url)

			// Настройка клиента с возможностью отключения проверки сертификата
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}

			client := &http.Client{Transport: tr}

			// Выполнение GET-запроса к сайту
			resp, err := client.Get(url)
			if err != nil {
				fmt.Println("Ошибка при выполнении запроса:", err, "\n")
				os.Exit(1)
			}
			defer resp.Body.Close()

			// Проверка наличия сертификата
			if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
				fmt.Println("Сертификат присутствует:")
				for _, cert := range resp.TLS.PeerCertificates {
					fmt.Println("Имя: ", cert.Subject.CommonName)
					fmt.Println("Организация: ", cert.Subject.Organization)
					fmt.Println("Издатель: ", cert.Issuer.CommonName)
					fmt.Println("Действителен с ", cert.NotBefore, " до ", cert.NotAfter)
					fmt.Println()
				}
			} else {
				fmt.Println("Сертификат отсутствует.\n")
			}
		case 4:
			// Завершение программы
			fmt.Println("Программа завершается.")
			return
		default:
			fmt.Println("Неверный вариант. Пожалуйста, выберите 1, 2 или 3.")
		}
	}
}
