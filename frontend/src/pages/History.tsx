import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiRequestError } from '../api'
import type { ApplicationStatus, ListItem } from '../types'
import styles from './History.module.css'

const PAGE_SIZE = 20

const STATUS_LABEL: Record<ApplicationStatus, string> = {
  generated: 'Готово',
  filtered_out: 'Отфильтровано',
  failed: 'Ошибка',
}

const STATUS_CLASS: Record<ApplicationStatus, string> = {
  generated: styles.badgeOk,
  filtered_out: styles.badgeNeutral,
  failed: styles.badgeError,
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

export default function History() {
  const [items, setItems] = useState<ListItem[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .listApplications(PAGE_SIZE, offset)
      .then((res) => {
        if (cancelled) return
        setItems(res.items)
        setTotal(res.total)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err instanceof ApiRequestError ? err.message : 'Не удалось загрузить историю')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [offset])

  return (
    <div className={styles.wrap}>
      <h1 className={styles.title}>История</h1>

      {loading && <div className={styles.hint}>Загрузка…</div>}
      {error && <div className={styles.error}>{error}</div>}

      {!loading && !error && items.length === 0 && (
        <div className={styles.hint}>Пока нет ни одного письма. Начните с новой вакансии.</div>
      )}

      <div className={styles.list}>
        {items.map((item) => (
          <Link key={item.id} to={`/applications/${item.id}`} className={styles.row}>
            <div className={styles.rowTop}>
              <span className={styles.rowTitle}>{item.title}</span>
              <span className={STATUS_CLASS[item.status]}>{STATUS_LABEL[item.status]}</span>
            </div>
            <div className={styles.rowMeta}>
              {item.company} · {formatDate(item.created_at)}
            </div>
            {item.preview && <div className={styles.rowPreview}>{item.preview}</div>}
          </Link>
        ))}
      </div>

      {total > PAGE_SIZE && (
        <div className={styles.pager}>
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            ← Назад
          </button>
          <span className={styles.pagerInfo}>
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} из {total}
          </span>
          <button
            type="button"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Вперёд →
          </button>
        </div>
      )}
    </div>
  )
}
