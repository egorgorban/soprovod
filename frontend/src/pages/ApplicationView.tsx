import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiRequestError } from '../api'
import type { ApplicationWithVacancy } from '../types'
import LetterCard from '../components/LetterCard'
import styles from './ApplicationView.module.css'

export default function ApplicationView() {
  const { id } = useParams<{ id: string }>()
  const [data, setData] = useState<ApplicationWithVacancy | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .getApplication(Number(id))
      .then((res) => {
        if (!cancelled) setData(res)
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof ApiRequestError ? err.message : 'Не удалось загрузить заявку')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id])

  return (
    <div className={styles.wrap}>
      <Link to="/history" className={styles.back}>
        ← История
      </Link>

      {loading && <div className={styles.hint}>Загрузка…</div>}
      {error && <div className={styles.error}>{error}</div>}

      {data && (
        <LetterCard
          application={data.application}
          vacancy={data.vacancy}
          onSaved={(application) => setData({ ...data, application })}
        />
      )}
    </div>
  )
}
