package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
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

//Get top stories from hacker-news api
func getNews(w http.ResponseWriter, r *http.Request) {
  resp, err := http.Get("https://hacker-news.firebaseio.com/v0/topstories.json")

  if err == nil {
  	defer resp.Body.Close()
  	body, read_err := ioutil.ReadAll(resp.Body)

  	if read_err == nil {

  		var arr []int
  		_ = json.Unmarshal([]byte(body), &arr)

  		jobs := make(chan int, 10)
  		results := make(chan string, 10)

  		//allow article retrieval be executed as two goroutines
  		go getArticle(jobs, results)
  		go getArticle(jobs, results)

  		for i := 0; i < 10; i++ {
  			jobs <- arr[i]
  		}
  		close(jobs)

  		w.Header().Set("Content-Type", "text/html")
  		fmt.Fprintf(w,"<h1>Hacker News</h1>")

  		for j := 0; j < 10; j++ {
  			article_markdown := <- results
  			fmt.Fprintf(w,"<p>%s</p>",article_markdown)
  		}

  	} else {
  		w.Write([]byte("Error retrieving stories"))
  	}

  } else {
  	w.Write([]byte("Error retrieving stories"))
  }

}

//Take article ids from job list, push html into results channel
func getArticle(jobs <-chan int, results chan<- string){
	for id := range jobs {
		article := makeArticleReq(id)
		results <- generateHTML(article)
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
			e := json.Unmarshal([]byte(body),&article)

			if e != nil {
				log.Fatal(e)
			}
			//show(article)
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

func show(x Article) {
	fmt.Printf("#%d \t %s \t By: %s \n",
		x.Id,
		x.Title,
		x.By,
	)
}


func main() {
  http.HandleFunc("/", getNews)
  if err := http.ListenAndServe(":8080", nil); err != nil {
    panic(err)
  }
}