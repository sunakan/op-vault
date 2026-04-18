# TODO

## `op-cache prune` サブコマンドの追加

1Password に存在しない ref のキャッシュエントリを削除するサブコマンド。

### 動作

1. キャッシュ済み ref 一覧を取得（`_list_refs`）
2. 各 ref に対して `op item get` を試みる
3. 失敗した ref のエントリをキーチェーンから削除
4. 削除した ref を stdout に表示

### 出力イメージ

```
削除しました: op://Personal/old-item/password
削除しました: op://Work/deleted-item/token
完了: 2 件削除, 3 件保持
```

### 注意事項

- 1Password がオフライン・セッション未確立の場合は実行しない（誤削除防止）
- `refresh` との住み分け: `prune` は「存在しない ref を削除」、`refresh` は「全件再取得して上書き」
