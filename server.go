package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
	"os"
)

//article struct allowing for unmarshalling
type Article struct {
	By string `json:"by"`
	Descendants int `json:"descendants"`
	Id int `json:"id"`
	Kids []int `json:"kids"`
	Score int `json:"score"`
	Time int `json:"time"`
	Title string `json:"title"`
	Text string `json:"text"`
	Type string `json:"type"`
	Url string `json:"url"`
}

type temperature struct {
  Value float64
  Unit string
}

type weather struct {
  DateTime, IconPhrase string
  Temperature temperature
  PrecipitationProbability int
}

type stopRTPI struct {
	ErrorCode string `json:"errorcode"`
	ErrorMessage string `json:"errormessage"`
	ResultCount int `json:"numberofresults"`
	StopId string `json:"stopid"`
	Time string `json:timestamp`
	Results []bus `json:results`
}

type bus struct {
	Eta string `json:"arrivaldatetime"`
	DueMinutes string `json:"duetime"`
	ScheduledArrival string `json:"scheduledarrivaldatetime"`
	Destination string `json:"destination"`
	Origin string `json:"origin"`
}

type Server struct {
	WeatherLocation int `json:"weatherLocation"`
	StoriesCount int `json:"storiesCount"`
	BusStops []int `json:"busStops"`
}

func buildServer() Server {
	config, err := os.Open("config.json")

	if err != nil {
		log.Fatal(err)
	}
	
	defer config.Close()
	content, read_err := ioutil.ReadAll(config)
	
	if read_err != nil {
		log.Fatal(read_err)
	}

	server := Server{}
	json_err := json.Unmarshal(content,&server)

	if json_err != nil {
		log.Fatal(json_err)
	}

	return server
}

func (server Server) runServer() {
	file_server := http.FileServer(http.Dir("pages/"))
	http.Handle("/", http.StripPrefix("/",file_server))

	http.HandleFunc("/news", server.getNews)
	http.HandleFunc("/weather", server.getWeather)
	http.HandleFunc("/bus", server.getBuses)
	if err := http.ListenAndServe(":8080", nil); err != nil {
    	panic(err)
	}
}

//Get top stories from hacker-news api
func (server Server) getNews(w http.ResponseWriter, r *http.Request) {
  resp, err := http.Get("https://hacker-news.firebaseio.com/v0/topstories.json")

  if err == nil {
  	defer resp.Body.Close()
  	body, read_err := ioutil.ReadAll(resp.Body)

  	if read_err == nil {

  		var arr []int
  		_ = json.Unmarshal(body, &arr)

  		articles := make([]Article, server.StoriesCount)

  		jobs := make(chan int, server.StoriesCount)
  		results := make(chan Article, server.StoriesCount)

  		//allow article retrieval be executed as two goroutines
  		go getArticle(jobs, results)
  		go getArticle(jobs, results)

  		for i := 0; i < server.StoriesCount; i++ {
  			jobs <- arr[i]
  		}
  		close(jobs)

  		w.Header().Set("Content-Type", "application/json")

  		for j := 0; j < server.StoriesCount; j++ {
  			article_markdown := <- results
  			articles[j] = article_markdown
  		}
  		json.NewEncoder(w).Encode(articles)

  	} else {
  		w.Write([]byte("Error retrieving stories"))
  	}

  } else {
  	w.Write([]byte("Error retrieving stories"))
  }

}

//Take article ids from job list, push html into results channel
func getArticle(jobs <-chan int, results chan<- Article){
	for id := range jobs {
		results <- makeArticleReq(id)
	}
}

func generateArticleURL(id int) string {
	return fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json?print=pretty",id)
}

func makeArticleReq(id int) Article {
	url := generateArticleURL(id)
	resp, err := http.Get(url)

	if err == nil {
		defer resp.Body.Close()
		body, read_err := ioutil.ReadAll(resp.Body)

		if read_err == nil {
			article := Article{}
			e := json.Unmarshal(body,&article)

			if e != nil {
				log.Fatal(e)
			}
			return article
		}
	}
	log.Fatal("Error Retrieving #%d",id)
	return Article{}
}

func generateHTML(article Article) string {
	hn_link := fmt.Sprintf("https://news.ycombinator.com/item?id=%d",article.Id)
	return fmt.Sprintf("<a href=\"%s\" target=\"_blank\">#%d: %s</a><br>- <i>%s</i><br>%d points<br><a href=\"%s\" target=\"_blank\">View on HN</a>",
		article.Url,
		article.Id,
		article.Title,
		article.By,
		article.Score,
		hn_link,
	)
}

func getAccuweatherKey() string{
	keyFile, err := os.Open("accuweather-key.txt")

	if err != nil {
		log.Fatal(err)
	}
	defer keyFile.Close()

	bytes, err := ioutil.ReadAll(keyFile)
	str_key := string(bytes[:])
	return str_key
}

func formatHour(h weather) string {
  hour := h.DateTime[11:16]
  return fmt.Sprintf("%s \t %s \t %.1f%s \t %d%%\n",
    hour,
    h.IconPhrase,
    h.Temperature.Value,
    h.Temperature.Unit,
    h.PrecipitationProbability,
  )
}

func (server Server) getWeather(w http.ResponseWriter, r *http.Request) {
	apiKey := getAccuweatherKey()

	weather_url := fmt.Sprintf("http://dataservice.accuweather.com/forecasts/v1/hourly/12hour/%d", server.WeatherLocation)

	req, req_err := http.NewRequest(
		"GET",
		weather_url,
		nil,
	)
	if req_err != nil{
		log.Fatal(req_err)
	}

	queries := req.URL.Query()
	queries.Add("apikey",apiKey)
	queries.Add("metric","true")
	queries.Add("details","true")

	req.URL.RawQuery = queries.Encode()

	resp, err := http.Get(req.URL.String())

	if err != nil{
		log.Fatal("Error retrieving weather")
	}

	defer resp.Body.Close()
	body, read_err := ioutil.ReadAll(resp.Body)

	if read_err != nil{
		log.Fatal("Error reading body")
	}

	var hours []weather

	e := json.Unmarshal(body, &hours)
	if e != nil {
		log.Fatal(e)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hours)
}

func getBusRTPIUrl(stop int) string {
	return fmt.Sprintf("https://data.smartdublin.ie/cgi-bin/rtpi/realtimebusinformation?stopid=%d", stop)
}

func (server Server) getBuses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	length := len(server.BusStops)
	payload := make([]stopRTPI,length)

	for i := 0; i < length; i++ {
		stop_num := server.BusStops[i]
		req_url := getBusRTPIUrl(stop_num)

		resp, resp_err := http.Get(req_url)

		if resp_err != nil {
			log.Fatal(resp_err)
		}

		defer resp.Body.Close()
		body, read_err := ioutil.ReadAll(resp.Body)

		if read_err != nil {
			log.Fatal(read_err)
		}

		rtpi := stopRTPI{}
		json_err := json.Unmarshal(body, &rtpi)

		if json_err != nil {
			log.Fatal(json_err)
		}

		payload[i] = rtpi
	}	
	json.NewEncoder(w).Encode(payload)
}

func main() {
  server := buildServer()
  server.runServer()
}