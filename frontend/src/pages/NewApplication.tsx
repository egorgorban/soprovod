import { useState, type FormEvent } from 'react'
import { api, ApiRequestError } from '../api'
import type { ApplicationWithVacancy } from '../types'
import LetterCard from '../components/LetterCard'
import styles from './NewApplication.module.css'

export default function NewApplication() {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ApplicationWithVacancy | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!url.trim() || loading) return
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const data = await api.createApplication(url.trim())
      setResult(data)
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : 'Что-то пошло не так, попробуйте ещё раз')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.wrap}>
      {!result && (
        <>
          <h1 className={styles.title}>Новое сопроводительное письмо</h1>
          <p className={styles.subtitle}>Вставьте ссылку на вакансию hh.ru — остальное сделает сервис.</p>

          <form className={styles.form} onSubmit={handleSubmit}>
            <input
              type="url"
              className={styles.input}
              placeholder="https://hh.ru/vacancy/123456"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              disabled={loading}
              required
            />
            <button type="submit" className={styles.button} disabled={loading || !url.trim()}>
              {loading ? 'Генерация…' : 'Сгенерировать письмо'}
            </button>
          </form>

          {loading && (
            <div className={styles.loading}>
              <span className={styles.spinner} aria-hidden />
              Читаю вакансию и пишу письмо…
            </div>
          )}

          {error && <div className={styles.error}>{error}</div>}
        </>
      )}

      {result && (
        <>
          <button
            type="button"
            className={styles.newBtn}
            onClick={() => {
              setResult(null)
              setUrl('')
            }}
          >
            ← Новое письмо
          </button>
          <LetterCard
            application={result.application}
            vacancy={result.vacancy}
            onSaved={(application) => setResult({ ...result, application })}
          />
        </>
      )}
    </div>
  )
}
