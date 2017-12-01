package main

import(
	"database/sql"
	
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	
	"github.com/davegardnerisme/phonegeocode"
)

var (
	countCountryCode = make(map[string]int)
)

func StringConverter(a []uint8) string{
	b := make([]byte, 0, len(a))
	for _, i := range a {
		b = append(b, byte(i))
	}
	return string(b)
}

func ParsePhoneNum(s string){
	
	country, err := phonegeocode.New().Country(s)
	if err != nil {
		log.Infof("Error:", "Can't Parse Phonenum :", s)
		return
	}

	countCountryCode[country]+=1
}

type collector struct {
	target string
	database string
	login string
	passwd string
	request string
}

func (c collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

func (c collector) Collect(ch chan<- prometheus.Metric){

	for country, _ := range countCountryCode {
		countCountryCode[country] = 0
	}
	
	conn := "postgres://"+c.login+":"+c.passwd+"@"+c.target+"/"+c.database+"?sslmode=disable"
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Infof("Error scraping target %s: %s", c.target, err)
		return
	}

	r, err := db.Query(c.request)
	if err != nil {
		log.Infof("Error : %s", err)
		return
	}
	
	defer db.Close()
	defer r.Close()
		
	for r.Next() {
		var str string
        if err = r.Scan(&str); err != nil {
                log.Fatal(err)
        }
        ParsePhoneNum(str)
	}
	if err := r.Err(); err != nil {
		    log.Fatal(err)
	}
	
	for country, val := range countCountryCode {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("count_user_per_country", "Count User Per Country", []string{"country"}, nil),
			prometheus.GaugeValue,
			float64(val),
			country)
	}
	
}
