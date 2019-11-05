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

That's all!

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

## アプリの変更

---?code=webapp/go/embed.go

### `categories` の埋め込み

アプリ内から変更できないテーブルなのでソースコードに埋め込む

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

```sql
CREATE TABLE `transaction_evidences` (
  `id` bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `seller_id` bigint NOT NULL,
  `buyer_id` bigint NOT NULL,
  `status` enum('wait_shipping', 'wait_done', 'done') NOT NULL,
  `item_id` bigint NOT NULL UNIQUE,
  `item_name` varchar(191) NOT NULL,
  `item_price` int unsigned NOT NULL,
  `item_description` text NOT NULL,
  `item_category_id` int unsigned NOT NULL,
  `item_root_category_id` int unsigned NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_item_id (`item_id`)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4;
```
