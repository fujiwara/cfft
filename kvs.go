package cfft

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfrontkeyvaluestore"
)

type KVSCmd struct {
	List   struct{}      `cmd:"" help:"list key values"`
	Get    *KVSGetCmd    `cmd:"" help:"get value of key"`
	Put    *KVSPutCmd    `cmd:"" help:"put value of key"`
	Delete *KVSDeleteCmd `cmd:"" help:"delete key"`
	Info   struct{}      `cmd:"" help:"show info of key value store"`

	Output string `short:"o" help:"output format (json, text)" default:"json" enum:"json,text"`
}

type KVSGetCmd struct {
	Key string `arg:"" help:"key name" required:""`
}

type KVSPutCmd struct {
	Key   string `arg:"" help:"key name" required:""`
	Value string `arg:"" help:"value" required:""`
}

type KVSDeleteCmd struct {
	Key string `arg:"" help:"key name" required:""`
}

func (app *CFFT) ManageKVS(ctx context.Context, op string, opt KVSCmd) error {
	switch op {
	case "list":
		return app.KVSList(ctx, opt)
	case "get":
		return app.KVSGet(ctx, opt)
	case "put":
		return app.KVSPut(ctx, opt)
	case "delete":
		return app.KVSDelete(ctx, opt)
	case "info":
		return app.KVSInfo(ctx, opt)
	default:
		return fmt.Errorf("unknown command %s", op)
	}
}

func (app *CFFT) KVSList(ctx context.Context, opt KVSCmd) error {
	p := cloudfrontkeyvaluestore.NewListKeysPaginator(app.cfkvs, &cloudfrontkeyvaluestore.ListKeysInput{
		KvsARN:     aws.String(app.cfkvsArn),
		MaxResults: aws.Int32(50),
	})
	buf := bufio.NewWriter(os.Stdout)
	for p.HasMorePages() {
		res, err := p.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list keys, %w", err)
		}
		for _, item := range res.Items {
			s, err := formatKVSItem(&KVSItem{Key: aws.ToString(item.Key), Value: aws.ToString(item.Value)}, opt.Output)
			if err != nil {
				return fmt.Errorf("failed to format item, %w", err)
			}
			buf.WriteString(s)
		}
	}
	return buf.Flush()
}

func (app *CFFT) KVSGet(ctx context.Context, opt KVSCmd) error {
	res, err := app.cfkvs.GetKey(ctx, &cloudfrontkeyvaluestore.GetKeyInput{
		KvsARN: aws.String(app.cfkvsArn),
		Key:    aws.String(opt.Get.Key),
	})
	if err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	s, err := formatKVSItem(&KVSItem{Key: aws.ToString(res.Key), Value: aws.ToString(res.Value)}, opt.Output)
	if err != nil {
		return fmt.Errorf("failed to format item, %w", err)
	}
	fmt.Fprint(os.Stdout, s)
	return nil
}

func (app *CFFT) KVSPut(ctx context.Context, opt KVSCmd) error {
	res, err := app.cfkvs.DescribeKeyValueStore(ctx, &cloudfrontkeyvaluestore.DescribeKeyValueStoreInput{
		KvsARN: aws.String(app.cfkvsArn),
	})
	if err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	if _, err := app.cfkvs.PutKey(ctx, &cloudfrontkeyvaluestore.PutKeyInput{
		KvsARN:  aws.String(app.cfkvsArn),
		IfMatch: res.ETag,
		Key:     aws.String(opt.Put.Key),
		Value:   aws.String(opt.Put.Value),
	}); err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	return nil
}

func (app *CFFT) KVSDelete(ctx context.Context, opt KVSCmd) error {
	res, err := app.cfkvs.DescribeKeyValueStore(ctx, &cloudfrontkeyvaluestore.DescribeKeyValueStoreInput{
		KvsARN: aws.String(app.cfkvsArn),
	})
	if err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	if _, err := app.cfkvs.DeleteKey(ctx, &cloudfrontkeyvaluestore.DeleteKeyInput{
		KvsARN:  aws.String(app.cfkvsArn),
		IfMatch: res.ETag,
		Key:     aws.String(opt.Delete.Key),
	}); err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	return nil
}

func (app *CFFT) KVSInfo(ctx context.Context, opt KVSCmd) error {
	res, err := app.cfkvs.DescribeKeyValueStore(ctx, &cloudfrontkeyvaluestore.DescribeKeyValueStoreInput{
		KvsARN: aws.String(app.cfkvsArn),
	})
	if err != nil {
		return fmt.Errorf("failed to get key, %w", err)
	}
	switch opt.Output {
	case "json":
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal kvs, %w", err)
		}
		fmt.Fprintln(os.Stdout, string(b))
	case "text":
		fmt.Fprintf(os.Stdout, "KvsARN\t%s\n", aws.ToString(res.KvsARN))
		fmt.Fprintf(os.Stdout, "ETag\t%s\n", aws.ToString(res.ETag))
		fmt.Fprintf(os.Stdout, "ItemCount\t%d\n", aws.ToInt32(res.ItemCount))
		fmt.Fprintf(os.Stdout, "TotalSizeInBytes\t%d\n", aws.ToInt64(res.TotalSizeInBytes))
		fmt.Fprintf(os.Stdout, "Created\t%s\n", res.Created.Format(time.RFC3339))
		fmt.Fprintf(os.Stdout, "LastModified\t%s\n", res.LastModified.Format(time.RFC3339))
	}
	return nil
}

type KVSItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func formatKVSItem(item *KVSItem, format string) (string, error) {
	switch format {
	case "json":
		b, err := json.Marshal(item)
		if err != nil {
			return "", fmt.Errorf("failed to marshal item, %w", err)
		}
		return string(b) + "\n", nil
	case "text":
		return fmt.Sprintf("%s\t%s\n", item.Key, item.Value), nil
	default:
		return "", fmt.Errorf("unknown format %s", format)
	}
}
