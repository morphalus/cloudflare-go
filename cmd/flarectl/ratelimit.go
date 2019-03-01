package main

import (
	"encoding/json"
	"fmt"
	"github.com/cloudflare/cloudflare-go"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	// "github.com/google/go-cmp/cmp"
	"strconv"
	"strings"
)

func formatRateLimit(rule cloudflare.RateLimit, command string) []string {
	statuses := make([]string, 0, len(rule.Match.Response.Statuses))
	for _, v := range rule.Match.Response.Statuses {
		statuses = append(statuses, strconv.Itoa(v))
	}

	res := []string{}

	switch command {
	case "list":
		res = []string{
			rule.ID,
			rule.Description,
			strconv.FormatBool(rule.Disabled),
			strings.Join(rule.Match.Request.Methods, ","),
			strings.Join(statuses, ","),
			rule.Action.Mode,
		}
	case "describe":
		res = []string{
			rule.ID,
			rule.Description,
			strconv.FormatBool(rule.Disabled),
			strings.Join(rule.Match.Request.Methods, ","),
			strings.Join(statuses, ","),
			strings.Join(rule.Match.Request.Schemes, ","),
			rule.Match.Request.URLPattern,
			strconv.Itoa(rule.Threshold),
			rule.Action.Mode,
			strconv.Itoa(rule.Period),
		}
	}

	return res
}

func ratelimitRules(c *cli.Context) {
	if err := checkEnv(); err != nil {
		fmt.Println(err)
		return
	}

	zoneID, err := api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return
	}
	pageOpts := cloudflare.PaginationOptions{
		PerPage: 5,
		Page:    1,
	}
	rateLimits, resultInfo, err := api.ListRateLimits(zoneID, pageOpts)
	if err != nil {
		log.Fatal(err)
	}

	// debug starts
	// fmt.Printf("%T / %+v\n\n", resultInfo, resultInfo)
	// debug ends

	// rules := make([]cloudflare.RateLimit, 0, resultInfo.Total)
	// rules = append(rules, rateLimits)

	if resultInfo.TotalPages > 1 {
		for page := 2; page <= resultInfo.TotalPages; page++ {
			pageOpts.Page = page
			r, _, err := api.ListRateLimits(zoneID, pageOpts)
			rateLimits = append(rateLimits, r...)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%T / %+v\n\n", rateLimits, rateLimits)
			// output = append(output, rateLimits)
		}
	}

	output := make([][]string, 0, len(rateLimits))
	for _, rule := range rateLimits {
		output = append(output, formatRateLimit(rule, c.Command.Name))
	}

	writeTable(output, "ID", "Description", "Disabled", "Methods", "Status", "Action")
}

func ratelimitRuleCreate(c *cli.Context) (err error) {
	if err := checkEnv(); err != nil {
		fmt.Println(err)
	}
	if err := checkFlags(c, "id", "zone"); err != nil {
		fmt.Println(err)
	}

	zoneID, err := api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return
	}

	var rateLimit cloudflare.RateLimit

	if c.Bool("stdin") {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println(err)
			return err
		}
		err = json.Unmarshal(data, &rateLimit)
		if err != nil {
			fmt.Println(err)
			return err
		}
	} else {
		rateLimit = cloudflare.RateLimit{
			Action: cloudflare.RateLimitAction{
				Mode:    c.String("action"),
				Timeout: c.Int("timeout"),
				Response: &cloudflare.RateLimitActionResponse{
					ContentType: c.String("response-content-type"),
					Body:        c.String("response-body"),
				},
			},
			Description: c.String("description"),
			Disabled:    c.Bool("disabled"),
			Match: cloudflare.RateLimitTrafficMatcher{
				Request: cloudflare.RateLimitRequestMatcher{
					Methods:    c.StringSlice("methods"),
					Schemes:    c.StringSlice("schemes"),
					URLPattern: c.String("url"),
				},
				Response: cloudflare.RateLimitResponseMatcher{
					Statuses:      c.IntSlice("status"),
					OriginTraffic: boolLambda,
				},
			},
			Threshold: c.Int("threshold"),
			Period:    c.Int("period"),
		}
	}

	zoneID, err = api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return
	}

	resp, err := api.CreateRateLimit(zoneID, rateLimit)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%T / %+v\n", resp, resp)

	// j, err := json.Marshal(rateLimit)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Println(string(j))

	return nil
}

func ratelimitRuleDelete(c *cli.Context) {
	if err := checkEnv(); err != nil {
		fmt.Println(err)
		return
	}
	if err := checkFlags(c, "id", "zone"); err != nil {
		fmt.Println(err)
		return
	}

	zoneID, err := api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = api.DeleteRateLimit(zoneID, c.String("id"))
	if err != nil {
		fmt.Println(err)
	}
}

func ratelimitRuleDescribe(c *cli.Context) (cloudflare.RateLimit, error) {
	zoneID, err := api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return cloudflare.RateLimit{}, err
	}

	rateLimit, err := api.RateLimit(zoneID, c.String("id"))
	if err != nil {
		fmt.Println(err)
		return cloudflare.RateLimit{}, err
	}

	if c.Bool("json-output") {
		output, err := json.Marshal(rateLimit)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("%s\n", string(output))

	} else {
		output := make([][]string, 0)
		output = append(output, formatRateLimit(rateLimit, c.Command.Name))

		writeTable(output, "ID", "Description", "Disabled", "Methods", "Status", "Schemes", "URL", "Threshold", "Action", "Period")
	}

	return rateLimit, nil
}

func ratelimitRuleUpdate(c *cli.Context) (err error) {
	zoneID, err := api.ZoneIDByName(c.String("zone"))
	if err != nil {
		fmt.Println(err)
		return
	} // attrs := make(map[string]interface{})

	var localRateLimit cloudflare.RateLimit
	// build local RateLimit
	if c.Bool("stdin") {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println(err)
			return err
		}
		err = json.Unmarshal(data, &localRateLimit)
		if err != nil {
			fmt.Println(err)
			return err
		}
	} else {
		localRateLimit = cloudflare.RateLimit{
			Action: cloudflare.RateLimitAction{
				Mode:    c.String("action"),
				Timeout: c.Int("timeout"),
				Response: &cloudflare.RateLimitActionResponse{
					ContentType: c.String("response-content-type"),
					Body:        c.String("response-body"),
				},
			},
			Description: c.String("description"),
			Disabled:    c.Bool("disabled"),
			Match: cloudflare.RateLimitTrafficMatcher{
				Request: cloudflare.RateLimitRequestMatcher{
					Methods:    c.StringSlice("methods"),
					Schemes:    c.StringSlice("schemes"),
					URLPattern: c.String("url"),
				},
				Response: cloudflare.RateLimitResponseMatcher{
					Statuses:      c.IntSlice("status"),
					OriginTraffic: boolLambda,
				},
			},
			Threshold: c.Int("threshold"),
			Period:    c.Int("period"),
		}
	}

	// build remote RateLimit
	remoteRateLimit, err := api.RateLimit(zoneID, c.String("id"))
	if err != nil {
		fmt.Println(err)
		return
	}

	areEqual := reflect.DeepEqual(localRateLimit, remoteRateLimit)
	fmt.Printf("%+v\n", areEqual)

	remoteRateLimitValue := reflect.ValueOf(remoteRateLimit)
	fmt.Printf("%T %+v\n", remoteRateLimitValue, remoteRateLimitValue)
	fmt.Printf("%T %+v\n", remoteRateLimitValue.Interface(), remoteRateLimitValue.Interface())
	fmt.Println("")
	// fmt.Printf("localRateLimit: (%T) %+v\n", localRateLimit, localRateLimit)
	// fmt.Println("")
	// fmt.Printf("remoteRateLimit: (%T) %+v\n", remoteRateLimit, remoteRateLimit)

	for k, v := range remoteRateLimitValue.Interface().(map[string]interface{}) {
		fmt.Printf("%v %v\n", k, v)
	}
	typ := remoteRateLimitValue.Type()
	fmt.Println(typ)
	// fmt.Printf("%T %+v\n", myMap, myMap)

	// for i := 0; i < remoteRateLimitValue.NumField(); i++ {
	// 	localRateLimitParams[i] = remoteRateLimitValue.Field(i).Interface()
	// }

	// fmt.Println("")
	// fmt.Printf("localRateLimit after: %+v\n", localRateLimitParams)

	// Create function to build local structure (rateLimit) than use DeepEqual
	// https://golang.org/pkg/reflect/#DeepEqual

	// remoteRateLimit, err := api.RateLimit(zoneID, c.String("id"))
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// fmt.Printf("Type: %T ; Value: %+v\n", remoteRateLimit, remoteRateLimit)
	// fmt.Println("")
	// fmt.Printf("Value of remoteRateLimit: %+v\n", reflect.ValueOf(remoteRateLimit))
	// fmt.Println("")

	// fmt.Printf("NumFields: %+v\n", reflect.ValueOf(remoteRateLimit).NumField())
	// fmt.Printf("NumFields: %T %+v\n", reflect.ValueOf(remoteRateLimit).Field(1), reflect.ValueOf(remoteRateLimit).Field(1))
	// fmt.Printf("NumFields: %T %+v\n", reflect.ValueOf(remoteRateLimit).Field(1).Interface(), reflect.ValueOf(remoteRateLimit).Field(1).Interface())
	// fmt.Println("")
	// fmt.Printf("NumFields: %T %+v\n", reflect.ValueOf(remoteRateLimit).FieldByName("ID").Interface(), reflect.ValueOf(remoteRateLimit).FieldByName("ID").Interface())
	// fmt.Printf("NumFields: %T %+v\n", reflect.ValueOf(remoteRateLimit).FieldByName("ID").Interface(), reflect.ValueOf(remoteRateLimit).FieldByName("ID").Interface())
	// fmt.Println("")

	// fmt.Printf("%+v\n", remoteRateLimit)
	// for i := range remoteRateLimit.([]interface{}) {
	// 	fmt.Printf("%+v\n", i)
	// }

	// if c.Bool("stdin") {
	// 	data, err := ioutil.ReadAll(os.Stdin)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		return err
	// 	}
	// 	err = json.Unmarshal(data, &rateLimit)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		return err
	// 	}
	// } else {
	// 	rateLimit = cloudflare.RateLimit{
	// 		Action: cloudflare.RateLimitAction{
	// 			Mode:    c.String("action"),
	// 			Timeout: c.Int("timeout"),
	// 			Response: &cloudflare.RateLimitActionResponse{
	// 				ContentType: c.String("response-content-type"),
	// 				Body:        c.String("response-body"),
	// 			},
	// 		},
	// 		Description: c.String("description"),
	// 		Disabled:    c.Bool("disabled"),
	// 		Match: cloudflare.RateLimitTrafficMatcher{
	// 			Request: cloudflare.RateLimitRequestMatcher{
	// 				Methods:    c.StringSlice("methods"),
	// 				Schemes:    c.StringSlice("schemes"),
	// 				URLPattern: c.String("url"),
	// 			},
	// 			Response: cloudflare.RateLimitResponseMatcher{
	// 				Statuses:      c.IntSlice("status"),
	// 				OriginTraffic: boolLambda,
	// 			},
	// 		},
	// 		Threshold: c.Int("threshold"),
	// 		Period:    c.Int("period"),
	// 	}

	return nil

	// fmt.Println()
	// r, err := api.UpdateRateLimit(zoneID, c.String("id"), localRateLimit)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return err
	// }

	// fmt.Printf("%+v\n", r)
	// return nil
	// ratelimitRuleDescribe()

	// rateLimit := cloudflare.RateLimit{
	// 	Action: cloudflare.RateLimitAction{
	// 		Mode:    c.String("action"),
	// 		Timeout: c.Int("timeout"),
	// 		Response: &cloudflare.RateLimitActionResponse{
	// 			ContentType: c.String("response-content-type"),
	// 			Body:        c.String("response-body"),
	// 		},
	// 	},
	// 	Description: c.String("description"),
	// 	Disabled:    c.Bool("disabled"),
	// 	Match: cloudflare.RateLimitTrafficMatcher{
	// 		Request: cloudflare.RateLimitRequestMatcher{
	// 			Methods:    c.StringSlice("methods"),
	// 			Schemes:    c.StringSlice("schemes"),
	// 			URLPattern: c.String("url"),
	// 		},
	// 		Response: cloudflare.RateLimitResponseMatcher{
	// 			Statuses:      c.IntSlice("status"),
	// 			OriginTraffic: boolLambda,
	// 		},
	// 	},
	// 	Threshold: c.Int("threshold"),
	// 	Period:    c.Int("period"),
	// }

	// resp, err := api.CreateRateLimit(zoneID, rateLimit)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// fmt.Printf("%T / %+v\n", resp, resp)

	// return nil
}
