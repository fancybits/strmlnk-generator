package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var (
	flagDir   = flag.String("dir", "Imports", "directory to place streamlinks")
	flagDebug = flag.Bool("debug", false, "debug with visible chrome window")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Printf("Usage: %v [-dir <dir>] <url> [<url>*]\n", os.Args[0])
		os.Exit(1)
	}

	log.Printf("[GEN] Launching chromium")
	u := launcher.New().
		UserDataDir("strmlnk-data").
		Headless(!*flagDebug).
		MustLaunch()

	log.Printf("[GEN] Connecting to chromium")
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	for _, arg := range flag.Args() {
		handlePage(browser, arg)
	}

	if *flagDebug {
		time.Sleep(600 * time.Second)
	}
}

func handlePage(browser *rod.Browser, lnk string) {
	uri, err := url.Parse(lnk)
	if err != nil {
		log.Printf("[ERR] Unrecognized URL: %v", err)
		return
	}

	log.Printf("[GEN] Loading page: %v", lnk)
	page := browser.MustPage(lnk).MustWaitLoad().MustWaitIdle()
	info := page.MustInfo()
	switch uri.Host {
	case "www.paramountplus.com":
		for _, e := range page.MustElements("section#latest-episodes a.link") {
			processParamountPlus(info, e)
		}
		if strings.HasSuffix(uri.Path, "/shows/tooning-out-the-news/") {
			for _, e := range page.MustElements(`section.js-le-carousel`) {
				if v := e.MustAttribute("data-title"); v != nil && *v == "Week in Review" {
					e.MustScrollIntoView()
					page.MustWaitRequestIdle()()
					for _, f := range e.MustElements("a.link") {
						processParamountPlus(info, f)
					}
				}
			}
		}
	case "www.hulu.com":
		for _, e := range page.MustElements(".EpisodeCollection__item") {
			processHulu(info, e)
		}
	default:
		log.Printf("[ERR] Unrecognized domain: %v", uri.Host)
	}
}

func processParamountPlus(info *proto.TargetTargetInfo, e *rod.Element) {
	href := e.MustAttribute("href")
	uri, _ := url.Parse(info.URL)
	res, _ := uri.Parse(*href)
	lnk := res.String()

	var name, season, episode string
	if data := e.MustAttribute("data-tracking"); data != nil {
		parts := strings.Split(*data, "|")
		name = parts[1]
		season = strings.TrimPrefix(parts[2], "S")
		episode = strings.TrimPrefix(parts[3], "Ep")
	} else if data := e.MustAttribute("aa-link"); data != nil {
		parts := strings.Split(*data, "|")
		name = parts[4]
		season = parts[6]
		episode = parts[7]
	} else {
		log.Printf("[ERR] Could not find paramountplus episode information for %v", *href)
		return
	}
	createEpisodeStreamLink(name, season, episode, lnk)
}

func processHulu(info *proto.TargetTargetInfo, e *rod.Element) {
	href := e.MustElement("a").MustAttribute("href")
	uri, _ := url.Parse(info.URL)
	res, _ := uri.Parse(*href)
	lnk := res.String()

	var name, season, episode string
	name = *e.MustElement(`meta[itemprop="partOfSeries"]`).MustAttribute("content")
	season = *e.MustElement(`meta[itemprop="partOfSeason"]`).MustAttribute("content")
	episode = *e.MustElement(`meta[itemprop="episodeNumber"]`).MustAttribute("content")

	createEpisodeStreamLink(name, season, episode, lnk)
}

func createEpisodeStreamLink(show, season, episode, lnk string) {
	path := filepath.Join(show, fmt.Sprintf("S%vE%v.strmlnk", season, episode))
	name := filepath.Join(*flagDir, "TV", path)
	log.Printf("[GEN] Generating stream link: %v", name)
	err := os.MkdirAll(filepath.Dir(name), 0755)
	if err != nil {
		log.Printf("[ERR] Failed to create directory: %v", err)
	}
	err = ioutil.WriteFile(name, []byte(lnk+"\n"), 0644)
	if err != nil {
		log.Printf("[ERR] Failed to create streamlink: %v", err)
	}
}
