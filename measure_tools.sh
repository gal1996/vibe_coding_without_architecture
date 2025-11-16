#!/bin/bash

# Vibe-Coding実験用 LOC変更行数計測ツール
# 使用方法: ./measure_tools.sh <path1> <path2>
#
# 説明: 2つのファイルまたはディレクトリ間の差分行数を計測します。
# 追加行数、削除行数、変更行数の合計を出力します。

# 引数チェック
if [ $# -ne 2 ]; then
    echo "使用方法: $0 <path1> <path2>"
    echo "  path1: 比較元のファイルまたはディレクトリ"
    echo "  path2: 比較先のファイルまたはディレクトリ"
    exit 1
fi

PATH1="$1"
PATH2="$2"

# パスの存在確認
if [ ! -e "$PATH1" ]; then
    echo "エラー: '$PATH1' が存在しません"
    exit 1
fi

if [ ! -e "$PATH2" ]; then
    echo "エラー: '$PATH2' が存在しません"
    exit 1
fi

# 両方がファイルの場合
if [ -f "$PATH1" ] && [ -f "$PATH2" ]; then
    echo "ファイル比較: $PATH1 vs $PATH2"
    echo "=================================="

    # diff統計を取得
    DIFF_OUTPUT=$(diff -u "$PATH1" "$PATH2" | grep -E "^[+\-]" | grep -v "^[+\-]{3}")

    # 追加行数（+で始まる行）
    ADDED=$(echo "$DIFF_OUTPUT" | grep "^+" | wc -l | tr -d ' ')

    # 削除行数（-で始まる行）
    DELETED=$(echo "$DIFF_OUTPUT" | grep "^-" | wc -l | tr -d ' ')

    # 変更行数の合計（追加＋削除）
    TOTAL=$((ADDED + DELETED))

    echo "追加行数: $ADDED"
    echo "削除行数: $DELETED"
    echo "変更行数合計 (LOC_churn): $TOTAL"

# 両方がディレクトリの場合
elif [ -d "$PATH1" ] && [ -d "$PATH2" ]; then
    echo "ディレクトリ比較: $PATH1 vs $PATH2"
    echo "=================================="

    # 一時ファイルを作成
    TEMP_FILE1=$(mktemp)
    TEMP_FILE2=$(mktemp)

    # ディレクトリ内のすべてのファイルをリスト化（隠しファイルを除く）
    find "$PATH1" -type f -not -path '*/\.*' | sort > "$TEMP_FILE1"
    find "$PATH2" -type f -not -path '*/\.*' | sort > "$TEMP_FILE2"

    # 相対パスに変換
    sed -i.bak "s|^$PATH1/||" "$TEMP_FILE1" 2>/dev/null || sed -i "" "s|^$PATH1/||" "$TEMP_FILE1"
    sed -i.bak "s|^$PATH2/||" "$TEMP_FILE2" 2>/dev/null || sed -i "" "s|^$PATH2/||" "$TEMP_FILE2"

    # 全体の統計を初期化
    TOTAL_ADDED=0
    TOTAL_DELETED=0

    # 共通ファイルの差分を計算
    COMMON_FILES=$(comm -12 "$TEMP_FILE1" "$TEMP_FILE2")

    for FILE in $COMMON_FILES; do
        if [ -f "$PATH1/$FILE" ] && [ -f "$PATH2/$FILE" ]; then
            DIFF_OUTPUT=$(diff -u "$PATH1/$FILE" "$PATH2/$FILE" 2>/dev/null | grep -E "^[+\-]" | grep -v "^[+\-]{3}")
            ADDED=$(echo "$DIFF_OUTPUT" | grep "^+" | wc -l | tr -d ' ')
            DELETED=$(echo "$DIFF_OUTPUT" | grep "^-" | wc -l | tr -d ' ')
            TOTAL_ADDED=$((TOTAL_ADDED + ADDED))
            TOTAL_DELETED=$((TOTAL_DELETED + DELETED))
        fi
    done

    # 新規ファイルの行数（PATH2にのみ存在）
    NEW_FILES=$(comm -13 "$TEMP_FILE1" "$TEMP_FILE2")
    for FILE in $NEW_FILES; do
        if [ -f "$PATH2/$FILE" ]; then
            LINES=$(wc -l < "$PATH2/$FILE" | tr -d ' ')
            TOTAL_ADDED=$((TOTAL_ADDED + LINES))
        fi
    done

    # 削除されたファイルの行数（PATH1にのみ存在）
    DELETED_FILES=$(comm -23 "$TEMP_FILE1" "$TEMP_FILE2")
    for FILE in $DELETED_FILES; do
        if [ -f "$PATH1/$FILE" ]; then
            LINES=$(wc -l < "$PATH1/$FILE" | tr -d ' ')
            TOTAL_DELETED=$((TOTAL_DELETED + LINES))
        fi
    done

    # 結果を表示
    TOTAL=$((TOTAL_ADDED + TOTAL_DELETED))

    echo "追加行数: $TOTAL_ADDED"
    echo "削除行数: $TOTAL_DELETED"
    echo "変更行数合計 (LOC_churn): $TOTAL"

    # 一時ファイルを削除
    rm -f "$TEMP_FILE1" "$TEMP_FILE1.bak" "$TEMP_FILE2" "$TEMP_FILE2.bak"

else
    echo "エラー: 両方のパスが同じタイプ（ファイルまたはディレクトリ）である必要があります"
    exit 1
fi

# CSV形式での出力オプション
echo ""
echo "CSV形式: $TOTAL"