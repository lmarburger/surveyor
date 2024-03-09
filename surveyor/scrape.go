package surveyor

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"strconv"
	"strings"
)

func Scrape(ctx context.Context) (SignalData, error) {
	req, reqErr := http.NewRequestWithContext(ctx, "GET", "http://192.168.100.1/cmSignalData.htm", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("error starting request: %w", reqErr)
	}
	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		return nil, fmt.Errorf("error fetching html: %w", respErr)
	}
	defer ClosePrintErr(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error, status=%d: %s", resp.StatusCode, resp.Status)
	}

	doc, parseErr := goquery.NewDocumentFromReader(resp.Body)
	if parseErr != nil {
		return nil, fmt.Errorf("error parsing html: %w", parseErr)
	}

	downstreamTable, downstreamErr := findTable("downstream", 0, doc)
	if downstreamErr != nil {
		return nil, downstreamErr
	}

	codewordsTable, codewordsErr := findTable("codewords", 3, doc)
	if codewordsErr != nil {
		return nil, codewordsErr
	}

	var channelIDs, frequencies, snRatios, powerLevels, unerroreds, correctables, uncorrectables []string
	var queryErr error

	downstreamTable.Each(func(_ int, table *goquery.Selection) {
		channelIDs = findChannelValues(table, 2)
		frequencyLabels := findChannelValues(table, 3)
		snRatioLabels := findChannelValues(table, 4)
		powerLevelLabels := findChannelValues(table, 6)

		if len(channelIDs) != totalChannels || len(frequencyLabels) != totalChannels || len(snRatioLabels) != totalChannels || len(powerLevelLabels) != totalChannels {
			queryErr = fmt.Errorf(
				"unexpected channels in downstream table, expected %d got %d: %v\n",
				totalChannels, len(channelIDs), channelIDs,
			)
			return
		}

		frequencies, queryErr = extractNumbers(frequencyLabels)
		if queryErr != nil {
			return
		}

		snRatios, queryErr = extractNumbers(snRatioLabels)
		if queryErr != nil {
			return
		}

		powerLevels, queryErr = extractNumbers(powerLevelLabels)
		if queryErr != nil {
			return
		}
	})

	if queryErr != nil {
		return nil, queryErr
	}

	codewordsTable.Each(func(_ int, table *goquery.Selection) {
		newChannelIDs := findChannelValues(table, 2)
		if !channelIDsEqual(channelIDs, newChannelIDs) {
			queryErr = fmt.Errorf("channels between tables not equal. first: %v second: %v\n", channelIDs, newChannelIDs)
			return
		}

		unerroredLabels := findChannelValues(table, 3)
		correctableLabels := findChannelValues(table, 4)
		uncorrectableLabels := findChannelValues(table, 5)

		if len(unerroredLabels) != totalChannels || len(correctableLabels) != totalChannels || len(uncorrectableLabels) != totalChannels {
			queryErr = fmt.Errorf(
				"unexpected channels in codewords table, expected %d got %d: %v\n",
				totalChannels, len(channelIDs), channelIDs,
			)
			return
		}

		unerroreds, queryErr = extractNumbers(unerroredLabels)
		if queryErr != nil {
			return
		}

		correctables, queryErr = extractNumbers(correctableLabels)
		if queryErr != nil {
			return
		}

		uncorrectables, queryErr = extractNumbers(uncorrectableLabels)
		if queryErr != nil {
			return
		}
	})

	if queryErr != nil {
		return nil, queryErr
	}

	channelData := make(SignalData)
	for i, channelID := range channelIDs {
		converted, convErr := strconv.Atoi(channelID)
		if convErr != nil {
			return nil, convErr
		}

		channelData[converted] = SignalDatum{
			Frequency:     frequencies[i],
			SNRatio:       snRatios[i],
			PowerLevel:    powerLevels[i],
			Unerrored:     unerroreds[i],
			Correctable:   correctables[i],
			Uncorrectable: uncorrectables[i],
		}
	}

	return channelData, nil
}

func channelIDsEqual(listA, listB []string) bool {
	if len(listA) != len(listB) {
		return false
	}

	for i, a := range listA {
		if a != listB[i] {
			return false
		}
	}

	return true
}

func extractNumbers(labels []string) ([]string, error) {
	var values []string
	for _, label := range labels {
		var value int
		_, err := fmt.Sscanf(label, "%d", &value)
		if err != nil {
			return nil, fmt.Errorf("error extracting number, got %s\n", label)
		}
		values = append(values, strconv.Itoa(value))
	}
	return values, nil
}

func findTable(name string, index int, doc *goquery.Document) (*goquery.Selection, error) {
	var err error
	table := doc.Find("table").Eq(index)
	if table.Length() != 1 {
		err = fmt.Errorf("didn't find %s table, expected 1 got %d\n", name, table.Length())
	}
	return table, err
}

func findChannelValues(table *goquery.Selection, rowIndex int) []string {
	var texts []string
	selector := fmt.Sprintf("tr:nth-of-type(%d) td:not(:first-of-type)", rowIndex)
	table.Find(selector).Each(func(i int, node *goquery.Selection) {
		texts = append(texts, strings.TrimSpace(node.Text()))
	})
	return texts
}
