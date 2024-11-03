package arweave

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/acsermely/veracy.server/src/common"
)

func Query(query string) ([]byte, error) {

	jsonData := map[string]string{
		"query": query,
	}
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return nil, err
	}
	resp, err := http.Post(common.ARWEAVE_URL+"/graphql", common.TX_APP_CONTENT_TYPE, bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("Error sending query:", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, err
	}

	return body, nil
}

func CheckPayment(sender string, tx string) (bool, error) {
	query := fmt.Sprintf(`{
		transactions(
			owners: ["%s"],
			tags: [
				{ name: "App-Name", values: ["%s"]},
				{ name: "Version", values: ["%s"]},
				{ name: "Type", values: ["%s"]},
				{ name: "Target", values: ["%s"]}
			]
		)
		{
			edges {
				node {
					id
				}
			}
		}
	}`, sender, common.TX_APP_NAME, common.TX_APP_VERSION, common.TX_TYPE_PAYMENT, tx)

	jsonData, err := Query(query)
	if err != nil {
		fmt.Println("Query error:", err)
		return false, err
	}

	var result common.ArQueryResult
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return false, nil
	}

	if len(result.Data.Transactions.Edges) > 0 {
		return true, nil
	}
	return false, nil
}

func IsDataPrivate(fullId string, tx string) (bool, error) {
	postData, err := getTxById(tx)
	if err != nil {
		return false, err
	}
	for _, content := range postData.Content {
		if content.Data == fullId {
			isPrivate := content.Privacy == common.TX_POST_PRIVACY_PRIVATE
			return isPrivate, nil
		}
	}
	return false, fmt.Errorf("ID not found")
}

func getTxById(txId string) (*common.Post, error) {
	response, err := http.Get(fmt.Sprintf("%s/%s", common.ARWEAVE_URL, txId))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var post common.Post
	if err := json.NewDecoder(response.Body).Decode(&post); err != nil {
		return nil, err
	}
	post.ID = txId
	return &post, nil
}
