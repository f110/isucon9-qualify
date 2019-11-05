### isuconの予選問題をチューニングする

#### ルール

- カツカツにチューニングしすぎない
  - あくまで本番運用できる状態を維持する
- 誰でも同様にセットアップしてスコアを出せるようにする
- 複数台構成は考慮する
- bcryptはそのまま。コストも変えない。

---

### 基本方針

- トランザクションは短くする
- 参照はmemcachedに逃がす
- 悲観的ロックはなるべく避ける
- ミドルウェアのチューニングはほぼしない

---

### 最終ベストスコア

```
2019/10/27 22:23:44 main.go:180: === final check ===
2019/10/27 22:23:44 main.go:212: 56600 0
```

**56600**

- MacBook Pro (15-inch, 2017)
- Core i7 2.8GHz (2C4T)
- macOS Mojave

---

### 構成

- Go 1.13
- MySQL 5.7
- memcached 1.5.10

---

### MySQLのチューニング

```
[mysqld]
innodb_buffer_pool_size = 1g
max_connections = 10000
```

- 全てのデータをメモリ上に載せて
- 十分なコネクションをできるようにする

---

### memcached

```
$ memcached -m 1024 -c 10240
```

- 確保する最大メモリ量を増やす
- コネクション数の上限も増やす
- バージョンは最新で良いが古くてもそんなに違わないはず

---

ミドルウェアの変更はベンチマーカーを動かすため

他にも1台構成で動かす場合はアプリとベンチマーカーそれぞれでulimitを上げておく必要がある（Linux/macOSに関わらず）

---

## yak shaving

スコアは変わらないが変更箇所を特定したりするためのもの

---

## Stackdriver Profilerを使おう

@fa[arrow-right]

---

### エンドポイントごとのレスポンスタイム

@fa[arrow-down]

+++

@snap[midpoint snap-100]
```go
func accessLog(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	if DisableAccessLog {
		return h
	} else {
		return func(w http.ResponseWriter, req *http.Request) {
			t1 := time.Now()
			h(w, req)

			log.Printf("method:%s path:%s duration:%v", req.Method, req.URL.Path, time.Now().Sub(t1))
		}
	}
}
```

@[2](グローバル変数でロギングを切れるようにしておく。このグローバル変数はこのメソッドが呼び出される前に初期化が終わっている)
@snapend

+++

@snap[midpoint snap-80]
```go
type mux struct {
	*goji.Mux
}

func (m *mux) HandleFunc(p goji.Pattern, h func(http.ResponseWriter, *http.Request)) {
	m.Mux.HandleFunc(p, accessLog(h))
}

func main() {
	mux := &mux{goji.NewMux()}

	// API
	mux.HandleFunc(pat.Post("/initialize"), postInitialize)
}
```

@[1-7](パスごとにaccessLogを挟むのが面倒なので一発で全てのエンドポイントに挟めるようする)
@snapend

---

### CPU Profile

コマンドライン引数をつけるだけでCPU Profileを取れるようにしておく

```console
$ go tool pprof -http=:8081 ./cpuprofile
```

このようにしてブラウザで閲覧することが多かった

@fa[arrow-down]

+++

```go
func main() {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
}
```

@[2](引数でcpuprofileを指定したときだけそのパスにプロファイル結果を吐き出すようしておく)

---

### シグナルハンドリング

ちゃんと終了しないとCPU Profileを取れないので

@fa[arrow-down]

+++

@snap[midpoint snap-90]
```go
func handleSignal(ctx context.Context, cancelFunc context.CancelFunc, ch chan os.Signal) {
	for {
		select {
		case <-ch:
			cancelFunc()
			return
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go handleSignal(ctx, cancelFunc, signalCh)
}
```
@snapend

---

### クエリの実行時間計測

クエリの実行時間をアプリサイドから計測してログとして出力する

@fa[arrow-down]

+++?code=webapp/go/querylog/querylog.go

+++

- `github.com/go-sql-driver/mysql` をwrapした自作。
- 一定以上時間がかかったクエリをログに出せる

---

### 便利な引数をつけると

フルオプション

```console
$ ./isucari -campaign 4 -disable-access-log -disable-query-log -cpuprofile ./cpuprofile
```

一番パフォーマンスが高い

```console
$ ./isucari -campaign 4 -disable-access-log -disable-query-log
```

もっと細かく時間を見たい

```console
$ ./isucari -campaign 0
```

※campaignを下げてアクセス数を減らしてログを少しでも見やすく

---

## アプリの変更

---

### `categories` の埋め込み

アプリ内から変更できないテーブルなのでソースコードに埋め込む

@fa[arrow-down]

+++?code=webapp/go/embed.go

@fa[arrow-down]

+++

```go
func init() {
	categoryMap = make(map[int]*Category)
	for _, v := range embedCategories {
		categoryMap[v.ID] = v
	}
	for _, v := range categoryMap {
		if v.ParentID != 0 {
			v.ParentCategoryName = categoryMap[v.ParentID].CategoryName
		}
	}

	categories = make([]Category, 0, len(categoryMap))
	for _, v := range categoryMap {
		categories = append(categories, *v)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].ID < categories[j].ID
	})
}
```

@[2-5](idでしか検索しないのでmapに詰め替えておく)
@[6-10](ParentCategoryNameは事前にデータを作っておく。ここまで含めて埋め込んでおいてもいい)
@[12-18](idの昇順で全件を返さないといけないので予めソートしておく)

---

### 全体的に

- 参照は全てcacheにオフロード
- レコードを変更したらcacheもupdateする

---

### `/initialize`

- 毎回キャッシュは全部消す
- DBへデータを流し込んだらキャッシュのwarm-upも行う

---

### `/new_items.json`

- 全カテゴリの新着アイテムを返す

@fa[arrow-down]

+++

### userのN+1

- cacheにオフロード
- 同一ユーザの参照は1リクエストに1回
  - リクエスト単位でオンメモリのキャッシュもしてる

---

### `/new_items/:root_category_id.json`

- 各カテゴリの出品と購入済みのリストを返す
- リストされるアイテムはそのままDBにクエリする

@fa[arrow-down]

+++

### userのN+1

- cacheにオフロード

---

### `/users/transactions.json`

- 自分が出品した or 購入したアイテムを返す

@fa[arrow-down]

+++

### トランザクション廃止

- 参照しかしてないのでトランザクションは使わない
- 自分のアイテムなので基本的にロックを取る必要がない

+++

### userのN+1

- cacheにオフロード

+++

### shipmentサービスの呼び出し

shipmentサービスにおけるstatusを確認するためにAPIを呼び出す

- `shippings` のstatusを見るとサービスの向こう側のステータスを確認すべきか分かる
  - `wait_pickup` `shipping` の時だけAPIを呼び出す

---

### `/items/edit`

- アイテムの編集ができる

@fa[arrow-down]

+++

### 悲観的ロックをなくす

- 悲観的ロックをなくす代わりにupdateのクエリにstatusも条件に加える
  - `WHERE id = ? AND status = ?`
  - 万全を期すなら price もクエリでチェックしたほうがいい
  - これでロックを使わずに安全に変更がアトミックになる

---

### `/buy`

- アイテムの購入

@fa[arrow-down]

+++

### 悲観的ロックの廃止

- 元のクエリは `items` をロックして多重購入を防いでる
- `items` をロックしてしまうと参照も待たされる（isolation level依存）
- 代わりに `transaction_evidences` をロックに使う

+++

### `transaction_evidences` によるロック

- `item_id` がユニーク制約なのでロック代わりになる
- 最初に購入できた人のみがinsertに成功する
- アイテム購入のトランザクションを
  - `transaction_evidences` へのinsert
  - `items` のupdate（buyerを購入者のidにする）
- と短くする

+++

### トランザクションを短くする

外部サービスの呼び出しはトランザクションの外で行う。

こうすることで購入できなかったユーザには高速にレスポンスを返せる。

購入できそうな人が購入に失敗した場合は適切にロールバックを行う必要がある。

- `transaction_evidences` のレコードを消す
- `items` を元に戻す

実際のサービスの場合、これらをユーザのリクエストコンテキストで行うと
コネクションが途中で切れた場合にロールバックが中途半端になるので対策を行う必要がある。

+++

### 外部サービスの呼び出しは同時に

- shipmentとpaymentの2つのサービスを呼び出さないといけないので同時に行う
- それぞれに800msecかかるので同時に呼び出しを開始する
- どちらかで失敗したらロールバックする
- なお **payment自体をロールバックする方法がない** のでそれは見なかったことにする

---

### `/ship`

- アイテムの発送準備（ピックアップ待ち）

@fa[arrow-down]

+++

### 悲観的ロックをなくす

- 楽観的ロックへ切り替え

---

### `/ship_done`

- アイテムの発送

@fa[arrow-down]

+++

### 悲観的ロックをなくす

- 楽観的ロックへ切り替え

---

### `/complete`

- 取引の完了

@fa[arrow-down]

+++

### 悲観的ロックをなくす

- 楽観的ロックへ切り替え

---

### インデックスの追加

```sql
CREATE TABLE `items` (
  `id` bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `seller_id` bigint NOT NULL,
  `buyer_id` bigint NOT NULL DEFAULT 0,
  `status` enum('on_sale', 'trading', 'sold_out', 'stop', 'cancel') NOT NULL,
  `name` varchar(191) NOT NULL,
  `price` int unsigned NOT NULL,
  `description` text NOT NULL,
  `image_name` varchar(191) NOT NULL,
  `category_id` int unsigned NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_category_id (`category_id`),
  INDEX idx_seller (`seller_id`, `created_at`),
  INDEX idx_buyer (`buyer_id`, `created_at`),
  INDEX idx_created_at (`created_at`)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
```

@[14-15](seller_idとbuyer_idで絞りcreated_atで並び替えるクエリ狙い)
@[16](created_atでソートされるクエリ狙い)

---

### クエリチューニング

- covering index等はほぼ使ってない
- 参照をmemcachedに逃がしているのでここで頑張るより逃がす方向を考える
- 不要なカラムを取らないのは有効
  - 特に日本語が入ってるカラム

---

### リファクタリング

- cacheを通してDBへクエリする場合は `***Repository` を通すようにする（main.goからの分離）
- memcachedはbinary protocolを使う
  - binary protocolの方がメモリ確保が少ない回数で行えるので高速
- シリアライザは gogoproto
  - `encoding/gob` は遅い。protobufの中でも最速の gogoproto でより速度を求める

---

### さらに上を目指すなら

- updateクエリを廃止してオブジェクトのsetterで対応したい（ORMの始まり）
  - 透過的にキャッシュを扱うには生クエリは生々しすぎる
- `/new_items/:id.json` のキャッシュ
  - キャッシュ機構は作れる。がシリアライザが遅くて不採用。
  - isuconの文脈においてなのでサービスとしてスケールさせるなら多少遅くてもアリ。
- bcryptのコストを下げる
  - しかしコストを下げるには一度正しいパスワードが必要なので全ユーザのログインを待たないといけない
- sessionの値を定義（ `encoding/gob` をやめる）

---

### まとめ

- 特に変なことはしてない
- 通常のハイパフォーマンスAPIサーバーと同じアーキテクチャを採用
- ロックを減らす・ロックの時間を短くする工夫を
- `pprof` で取ったスタックトレースをflamegraphにしたものは役に立つ
- なんでも詰め込める枠組みはあまり速くない。分かっているのであれば定義したほうがパフォーマンス上有利。
