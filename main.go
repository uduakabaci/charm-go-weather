package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type Weather struct {
	Timelines struct {
		Daily []struct {
			Time   string `json:"time"`
			Values struct {
				TemperatureAvg float64 `json:"temperatureAvg"`
				HumidityAvg    float64 `json:"humidityAvg"`
			} `json:"values"`
		} `json:"daily"`
	} `json:"timelines"`
}

type WeatherMsg struct {
	w   Weather
	err error
}

func (w *Weather) Decode(data []byte) error {
	err := json.Unmarshal(data, &w)
	if err != nil {
		return err
	}
	return nil
}

type Model struct {
	input        textinput.Model
	w            Weather
	table        table.Model
	currentCity  string
	updating     bool
	gettingInput bool
	mu           sync.Mutex
	spinner      spinner.Model
}

func (m *Model) Init() tea.Cmd {
	s := spinner.New()
	input := textinput.New()
	input.Placeholder = "Enter city name"
	input.CharLimit = 100
	input.Width = 50
	m.input = input
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.spinner = s
	// Ask for city on startup
	return func() tea.Msg {
		return tea.KeyMsg{Type: tea.KeyCtrlI}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case WeatherMsg:
		if msg.err != nil {
			return m, nil
		}
		m.w = msg.w
		m.InitTable()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlR:
			return m, m.LoadWeather(m.currentCity)
		case tea.KeyCtrlI:
			m.gettingInput = true
			m.input.Focus()
			return m, nil
		case tea.KeyEnter:
			if m.gettingInput {
				m.gettingInput = false
				return m, m.LoadWeather(m.input.Value())
			}
		}
	}

	if m.gettingInput {
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, cmd
}

func (m *Model) View() string {
	view := ""
	if m.updating {
		view = fmt.Sprintf("\n\n   %s Loading weather data...", m.spinner.View())
	}

	if m.gettingInput {
		view = fmt.Sprintf("Enter a city to see weather infor: \n\n%s\n\n", m.input.View()) + "\n"
	}

	if !m.gettingInput && !m.updating {
		view = "\nShowing weather data for " + m.currentCity + " \n" + baseStyle.Render(m.table.View())
	}

	return fmt.Sprintf("%s\n\n\nPress ctrl+c to quit, ctrl+r to refresh, ctrl+i to change city", view)
}

func (m *Model) LoadWeather(city string) tea.Cmd {
	return func() tea.Msg {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.updating {
			fmt.Println("Already fetching weather data")
			return WeatherMsg{Weather{}, fmt.Errorf("Already fetching weather data")}
		}

		m.updating = true
		defer func() { m.updating = false }()

		body, err := FetchWeather(city)
		if err != nil {
			fmt.Println(err)
			return WeatherMsg{err: err}
		}

		weather := Weather{}
		err = weather.Decode(body)
		if err != nil {
			fmt.Println(err)
			return WeatherMsg{err: err}
		}

		m.w = weather
		m.currentCity = city
		return WeatherMsg{w: weather}
	}
}

func (m *Model) InitTable() {
	m.table = table.New()
	rows := []table.Row{}
	columns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Temperature", Width: 15},
		{Title: "Humidity", Width: 10},
	}

	for _, day := range m.w.Timelines.Daily {
		rows = append(rows, []string{
			day.Time[:10],
			fmt.Sprintf("%.2f°C", day.Values.TemperatureAvg),
			fmt.Sprintf("%.2f%%", day.Values.HumidityAvg),
		})
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(9),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(false)
	t.SetStyles(s)

	m.table = t
}

func main() {
	m := Model{}
	go m.LoadWeather("uyo")
	m.InitTable()
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}
	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func FetchWeather(city string) ([]byte, error) {
	res, err := http.Get("https://api.tomorrow.io/v4/weather/forecast?location=" + url.QueryEscape(city) + "&apikey=BmOCo3WGWx0GwXXnfwcXcqPuyPN4VLor")
	if err != nil {
		return []byte{}, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()

	if err != nil {
		return []byte{}, err
	}

	return body, nil
}
