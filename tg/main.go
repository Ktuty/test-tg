package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

type Task struct {
	UserID       int64     `json:"user_id"`
	Content      string    `json:"content"`
	ReminderTime time.Time `json:"reminder_time"`
}

type User struct {
	UserID   int64  `json:"user_id"`
	ChatID   int64  `json:"chat_id"`
	Username string `json:"username"`
}

var userMessages = make(map[int64]string)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set in .env file")
	}
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	// Восстановление задач при запуске бота
	restoreTasks(bot)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() && update.Message.Command() == "ctrl" {
			handleCommand(update.Message, bot)
		} else {
			userMessages[update.Message.Chat.ID] = update.Message.Text
		}
	}
}

func handleCommand(message *tgbotapi.Message, bot *tgbotapi.BotAPI) {
	re := regexp.MustCompile(`@\w+ ctrl (\d+)([hdwm])`)
	matches := re.FindStringSubmatch(message.Text)
	if len(matches) != 3 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Неверный формат команды. Используйте @bot_name ctrl NM, где N - интервал, M - продолжительность (h, d, w, m)."))
		return
	}

	interval, err := strconv.Atoi(matches[1])
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ошибка при обработке интервала."))
		return
	}

	duration := matches[2]
	durationText := ""
	var durationTime time.Duration
	switch duration {
	case "h":
		durationText = "часов"
		durationTime = time.Hour * time.Duration(interval)
	case "d":
		durationText = "дней"
		durationTime = time.Hour * 24 * time.Duration(interval)
	case "w":
		durationText = "недель"
		durationTime = time.Hour * 24 * 7 * time.Duration(interval)
	case "m":
		durationText = "месяцев"
		durationTime = time.Hour * 24 * 30 * time.Duration(interval)
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Неверный формат продолжительности. Используйте h, d, w, m."))
		return
	}

	// Проверка, есть ли предыдущее сообщение
	if _, ok := userMessages[message.Chat.ID]; !ok {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Нет предыдущего сообщения для создания задачи."))
		return
	}

	// Отправка ответа пользователю
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("#Задача# принята. Напомню о ней через %d %s.", interval, durationText)))

	// Сохранение пользователя в базе данных
	user := User{
		ChatID:   message.Chat.ID,
		Username: message.From.UserName,
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		log.Println("Error marshaling user: ", err)
		return
	}

	resp, err := http.Post("http://localhost:8080/save_user", "application/json", bytes.NewBuffer(userJSON))
	if err != nil {
		log.Println("Error sending user to server: ", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Error saving user: ", resp.Status)
		return
	}

	var userResponse map[string]int64
	err = json.NewDecoder(resp.Body).Decode(&userResponse)
	if err != nil {
		log.Println("Error decoding user response: ", err)
		return
	}

	// Сохранение задачи в базе данных через сервер
	task := Task{
		UserID:       userResponse["user_id"],
		Content:      userMessages[message.Chat.ID],
		ReminderTime: time.Now().Add(durationTime),
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Println("Error marshaling task: ", err)
		return
	}

	resp, err = http.Post("http://localhost:8080/save_task", "application/json", bytes.NewBuffer(taskJSON))
	if err != nil {
		log.Println("Error sending task to server: ", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Error saving task: ", resp.Status)
		return
	}

	// Создание таймера
	time.AfterFunc(durationTime, func() {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Напоминание: #Задача#"))
	})
}

func restoreTasks(bot *tgbotapi.BotAPI) {
	resp, err := http.Get("http://localhost:8080/get_tasks")
	if err != nil {
		log.Println("Error getting tasks from server: ", err)
		return
	}
	defer resp.Body.Close()

	var tasks []Task
	err = json.NewDecoder(resp.Body).Decode(&tasks)
	if err != nil {
		log.Println("Error decoding tasks: ", err)
		return
	}

	for _, task := range tasks {
		bot.Send(tgbotapi.NewMessage(task.UserID, fmt.Sprintf("Напоминание: %s", task.Content)))
	}
}
