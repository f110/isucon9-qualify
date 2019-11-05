### isuconの予選問題をチューニングする

#### マイルール

- カツカツにチューニングしすぎない
  - あくまで本番運用できる状態を維持する
- 誰でも同様にセットアップしてスコアを出せるようにする
- 複数台構成は考慮する

---

### 基本方針

- トランザクションは短くする
- 参照はmemcachedに逃がす
- 悲観的ロックはなるべく避ける
- ミドルウェアのチューニングはほぼしない

---

### 最終ベストスコア

```
2019/10/27 22:42:54 main.go:180: === final check ===
2019/10/27 22:42:54 main.go:212: 61240 0
{"pass":true,"score":61240,"campaign":5,"language":"Go","messages":[]}
```

**61240**

---

### 構成

- Go 1.13
- MySQL 5.7

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

ミドルウェアの変更はベンチマーカーを動かすためのものがほぼ全て

他にも1台構成で動かす場合はアプリとベンチマーカーそれぞれでulimitを上げておく必要がある（Linux/macOSに関わらず）

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
