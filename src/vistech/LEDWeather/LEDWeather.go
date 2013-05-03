package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"visoline/ini"
	"visoline/mahonia"
	"vistech/odbc"
)

type AppConfig struct {
	Port       string
	ViewPath   string
	StaticPath string
	Server     string
	UserName   string
	Password   string
	DbName     string
}

var config = new(AppConfig)
var (
	view      *template.Template
	viewFuncs = template.FuncMap{
		"fs": func(t time.Time) string {
			return t.Format("2006-01-02 15:04")
		},
		"fd": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
	}
)

func main() {
	go SendLEDTask()
	http.HandleFunc(config.StaticPath, Static)
	http.HandleFunc("/", Index)
	http.HandleFunc("/LEDListTree", LEDListTree)
	http.HandleFunc("/LEDListTreeAll", LEDListTreeAll)
	http.HandleFunc("/WeatherSetting", WeatherSetting)
	http.HandleFunc("/SaveWeatherSetting", SaveWeatherSetting)
	log.Fatal(http.ListenAndServe(config.Port, nil))
}
func WeatherSetting(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		view.ExecuteTemplate(w, "weatherSetting", nil)
	}
}
func SaveWeatherSetting(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		data := r.FormValue("data")
		var editLeds = make([]Led, 0)
		err := json.Unmarshal([]byte(data), &editLeds)
		if err != nil {
			fmt.Println(err)
			return
		}
		var newLeds = make([]Led, 0)
		for _, v1 := range Leds {
			for _, v2 := range editLeds {
				if v1.Num == v2.Num {
					v1.IsSend = v2.IsSend
				}
			}
			newLeds = append(newLeds, v1)
		}
		Leds = newLeds
		SavaLeds()
	}
}
func LEDListTreeAll(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		LoadLeds()
		content, err := json.Marshal(Leds)
		if err != nil {
			fmt.Println(err)
			return
		}
		w.Write(content)
	}
}
func LEDListTree(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		LoadLeds()
		var ListTrees = make([]ListTree, 0)
		ListTrees = append(ListTrees, ListTree{Id: "base", Text: "龙湾发布气象LED大屏"})
		for _, v := range Leds {
			if v.IsSend == 1 {
				ListTrees = append(ListTrees, ListTree{Id: v.Num, Text: v.Name, Pid: "base"})
			}
		}
		content, err := json.Marshal(ListTrees)
		if err != nil {
			fmt.Println(err)
			return
		}
		w.Write(content)
	}
}

type ListTree struct {
	Id   string `json:"id"`
	Text string `json:"text"`
	Pid  string `json:"pid"`
}

//绘制等值线数据服务 demo
func Index(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page := make(map[string]interface{}, 0)
		wi := GetWeatherInfo()
		wim := GetWeatherInfoMore()
		page["Weatherinfo"] = wi
		page["WeatherinfoMore"] = wim
		page["weather"] = fmt.Sprintf("今天是%s %s 当前(%s)气温%s℃,%s%s；明天气温%s,%s,%s;后天气温%s,%s,%s。", wim.Winfom.Date_y, wim.Winfom.Week, wi.Winfo.Time, wi.Winfo.Temp, wi.Winfo.WD, wi.Winfo.WS, wim.Winfom.Temp2, wim.Winfom.Weather2, wim.Winfom.Wind2, wim.Winfom.Temp3, wim.Winfom.Weather3, wim.Winfom.Wind3)
		view.ExecuteTemplate(w, "weather", page)
	}
}
func SendLEDTask() {
	time.AfterFunc(1*time.Hour, func() { SendLEDTask() })
	LoadLeds()
	wi := GetWeatherInfo()
	wim := GetWeatherInfoMore()
	weather := fmt.Sprintf("今天是%s %s 当前(%s)气温%s℃,%s%s；明天气温%s,%s,%s;后天气温%s,%s,%s。", wim.Winfom.Date_y, wim.Winfom.Week, wi.Winfo.Time, wi.Winfo.Temp, wi.Winfo.WD, wi.Winfo.WS, wim.Winfom.Temp2, wim.Winfom.Weather2, wim.Winfom.Wind2, wim.Winfom.Temp3, wim.Winfom.Weather3, wim.Winfom.Wind3)
	conn, err := odbc.Connect(fmt.Sprintf("DSN=%s;UID=%s;PWD=%s", config.Server, config.UserName, config.Password))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	for _, led := range Leds {
		if led.IsSend == 1 {
			stmt, err := conn.ExecDirect(fmt.Sprintf("INSERT INTO ST_LED_SEND_RECORD(sender,reciver,content,resolution,color) VALUES('%s','%s','%s','%s',%s)", "气象", led.Num, weather, led.Resolution, led.Color))
			if err != nil {
				fmt.Println(err)
				return
			}
			stmt.Close()
		}
	}
	fmt.Println("SendLEDTask Over!")
}
func GetWeatherInfoMore() WeatherinfoMore {
	winfom := WeatherinfoMore{
		Winfom: WiMore{},
	}
	req, err := http.NewRequest("GET", fmt.Sprint("http://m.weather.com.cn/data/101210701.html"), nil)
	if err != nil {
		fmt.Println(err)
		return winfom
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1438.7 Safari/537.33")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return winfom
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return winfom
	}
	err = json.Unmarshal(body, &winfom)
	return winfom
}
func GetWeatherInfo() Weatherinfo {
	winfo := Weatherinfo{
		Winfo: Wi{},
	}
	req, err := http.NewRequest("GET", fmt.Sprint("http://www.weather.com.cn/data/sk/101210701.html"), nil)
	if err != nil {
		fmt.Println(err)
		return winfo
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1438.7 Safari/537.33")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return winfo
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return winfo
	}
	err = json.Unmarshal(body, &winfo)
	if err != nil {
		fmt.Println(err)
		return winfo
	}
	return winfo
}

//绘制等值线服务
func CallCenterWebApi(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		content := r.FormValue("content")
		telephones := r.FormValue("telephones")
		tels := strings.Split(telephones, ",")
		var data []byte
		buf := bytes.NewBuffer(data)
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s", "序号", "电话号码", "允许外呼通道", "月", "日", "积分"))
		//回车换行符
		buf.WriteByte('\r')
		buf.WriteByte('\n')
		for k, v := range tels {
			buf.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s", k+1, v, "", "1", "1", "100"))
			buf.WriteByte('\r')
			buf.WriteByte('\n')
		}
		//编码转换，utf-8转换到GBK
		encode := mahonia.NewEncoder("GBK")
		encodeData := encode.ConvertString(string(buf.Bytes()))
		result := []byte(encodeData)

		t := time.Now().Add(60 * time.Second)
		id := fmt.Sprintf("2-%s-%s", t.Format("20060102"), t.Format("150405.000"))
		id = strings.Replace(id, ".", "", 1)
		ioutil.WriteFile(fmt.Sprintf(`CallTask\%s.csv`, id), result, 0600)
		conn, err := odbc.Connect(fmt.Sprintf("DSN=%s;UID=%s;PWD=%s", config.Server, config.UserName, config.Password))
		if err != nil {
			fmt.Println(err)
			fmt.Fprint(w, err)
		}
		stmt, err := conn.ExecDirect(fmt.Sprintf("INSERT INTO YZ_GroupCall(Name,TheDate,TheTime,CallType,Content,Status1,Status2,TelAtt) VALUES('%s','%s','%s','%s','%s','%s','%s','%s.csv')", t.Format("200601021504.000"), t.Format("2006-01-02"), t.Format("15:04"), "2", content, "1", "0", id))
		if err != nil {
			fmt.Println(err)
			fmt.Fprint(w, err)
		}
		stmt.Close()
		conn.Close()
		fmt.Fprint(w, "ok")
	}
}
func init() {
	cfg, err := ini.Load("LEDWeather.ini", false)
	if err != nil {
		panic("LEDWeather.ini not find")
	}
	appSetting, ok := cfg.Sections["程序设置"]
	if !ok {
		panic("LEDWeather.ini setting error")
	}
	config.Port = appSetting.Pairs["端口"]
	config.ViewPath = appSetting.Pairs["模板文件"]
	config.StaticPath = appSetting.Pairs["静态文件"]
	config.Server = appSetting.Pairs["DSN名称"]
	config.UserName = appSetting.Pairs["数据库用户名"]
	config.Password = appSetting.Pairs["数据库密码"]
	absViewPath, err := filepath.Abs(config.ViewPath)
	if err != nil {
		panic(err)
	}
	view, err = template.New("view").Funcs(viewFuncs).ParseGlob(absViewPath)
	if err != nil {
		panic(err)
	}
}

//静态文件服务
func Static(w http.ResponseWriter, r *http.Request) {
	absPath, err := filepath.Abs(r.URL.Path)
	log.Println(absPath, r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
	}
	http.ServeFile(w, r, absPath)
}

type Led struct {
	Num        string
	Name       string
	Resolution string
	Color      string
	IsSend     int
}

var Leds = make([]Led, 0)

func SavaLeds() {
	b, err := json.Marshal(Leds)
	if err != nil {
		fmt.Println(err)
		return
	}
	ioutil.WriteFile("led.json", b, 0600)
}
func LoadLeds() {
	var data []byte
	var err error
	if data, err = ioutil.ReadFile("led.json"); err != nil {
		return
	}
	err = json.Unmarshal(data, &Leds)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("LoadLeds Success!")
}

type Wi struct {
	City    string `json:"city"`
	Cityid  string `json:"cityid"`
	Temp    string `json:"temp"`
	WD      string
	WS      string
	SD      string
	WSE     string
	Time    string `json:"time"`
	IsRadar string `json:"isRadar"`
	Radar   string `json:"radar"`
}
type Weatherinfo struct {
	Winfo Wi `json:"weatherinfo"`
}
type WeatherinfoMore struct {
	Winfom WiMore `json:"weatherinfo"`
}
type WiMore struct {
	City             string `json:"city"`
	City_en          string `json:"city_en"`
	Date_y           string `json:"date_y"`
	Date             string `json:"date"`
	Week             string `json:"week"`
	Fchh             string `json:"fchh"`
	Cityid           string `json:"cityid"`
	Temp1            string `json:"temp1"`
	Temp2            string `json:"temp2"`
	Temp3            string `json:"temp3"`
	Temp4            string `json:"temp4"`
	Temp5            string `json:"temp5"`
	Temp6            string `json:"temp6"`
	TempF1           string `json:"tempF1"`
	TempF2           string `json:"tempF2"`
	TempF3           string `json:"tempF3"`
	TempF4           string `json:"tempF4"`
	TempF5           string `json:"tempF5"`
	TempF6           string `json:"tempF6"`
	Weather1         string `json:"weather1"`
	Weather2         string `json:"weather2"`
	Weather3         string `json:"weather3"`
	Weather4         string `json:"weather4"`
	Weather5         string `json:"weather5"`
	Weather6         string `json:"weather6"`
	Img1             string `json:"img1"`
	Img2             string `json:"img2"`
	Img3             string `json:"img3"`
	Img4             string `json:"img4"`
	Img5             string `json:"img5"`
	Img6             string `json:"img6"`
	Img7             string `json:"img7"`
	Img8             string `json:"img8"`
	Img9             string `json:"img9"`
	Img10            string `json:"img10"`
	Img11            string `json:"img11"`
	Img12            string `json:"img12"`
	Img_single       string `json:"img_single"`
	Img_title1       string `json:"img_title1"`
	Img_title2       string `json:"img_title2"`
	Img_title3       string `json:"img_title3"`
	Img_title4       string `json:"img_title4"`
	Img_title5       string `json:"img_title5"`
	Img_title6       string `json:"img_title6"`
	Img_title7       string `json:"img_title7"`
	Img_title8       string `json:"img_title8"`
	Img_title9       string `json:"img_title9"`
	Img_title10      string `json:"img_title10"`
	Img_title11      string `json:"img_title11"`
	Img_title12      string `json:"img_title12"`
	Img_title_single string `json:"img_title_single"`
	Wind1            string `json:"wind1"`
	Wind2            string `json:"wind2"`
	Wind3            string `json:"wind3"`
	Wind4            string `json:"wind4"`
	Wind5            string `json:"wind5"`
	Wind6            string `json:"wind6"`
	Fx1              string `json:"fx1"`
	Fx2              string `json:"fx2"`
	Fl1              string `json:"fl1"`
	Fl2              string `json:"fl2"`
	Fl3              string `json:"fl3"`
	Fl4              string `json:"fl4"`
	Fl5              string `json:"fl5"`
	Fl6              string `json:"fl6"`
	Index            string `json:"index"`
	Index_d          string `json:"index_d"`
	Index48          string `json:"index48"`
	Index48_d        string `json:"index48_d"`
	Index_uv         string `json:"index_uv"`
	Index48_uv       string `json:"index48_uv"`
	Index_xc         string `json:"index_xc"`
	Index_tr         string `json:"index_tr"`
	Index_co         string `json:"index_co"`
	St1              string `json:"st1"`
	St2              string `json:"st2"`
	St3              string `json:"st3"`
	St4              string `json:"st4"`
	St5              string `json:"st5"`
	St6              string `json:"st6"`
	Index_cl         string `json:"index_cl"`
	Index_ls         string `json:"index_ls"`
	Index_ag         string `json:"index_ag"`
}
