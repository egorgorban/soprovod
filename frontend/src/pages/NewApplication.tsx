import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, ApiRequestError } from '../api'
import type { ApplicationWithVacancy } from '../types'
import LetterCard from '../components/LetterCard'
import styles from './NewApplication.module.css'

type Mode = 'url' | 'text'

const MODE_STORAGE_KEY = 'soprovod:new-application-mode'

function loadMode(): Mode {
  try {
    const v = localStorage.getItem(MODE_STORAGE_KEY)
    return v === 'text' ? 'text' : 'url'
  } catch {
    return 'url'
  }
}

function saveMode(mode: Mode) {
  try {
    localStorage.setItem(MODE_STORAGE_KEY, mode)
  } catch {
    // ignore
  }
}

function AutoTextarea({
  value,
  onChange,
  placeholder,
  disabled,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  disabled?: boolean
}) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }, [value])

  return (
    <textarea
      ref={ref}
      className={styles.textarea}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      rows={8}
    />
  )
}

export default function NewApplication() {
  const [mode, setMode] = useState<Mode>(loadMode)
  const [url, setUrl] = useState('')
  const [text, setText] = useState('')
  const [title, setTitle] = useState('')
  const [company, setCompany] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ApplicationWithVacancy | null>(null)

  function handleModeChange(next: Mode) {
    setMode(next)
    saveMode(next)
  }

  const canSubmit = mode === 'url' ? url.trim() !== '' : text.trim() !== ''

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!canSubmit || loading) return
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const data =
        mode === 'url'
          ? await api.createApplication({ url: url.trim() })
          : await api.createApplication({
              text: text.trim(),
              title: title.trim() || undefined,
              company: company.trim() || undefined,
            })
      setResult(data)
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : 'Что-то пошло не так, попробуйте ещё раз')
    } finally {
      setLoading(false)
    }
  }

  function resetForm() {
    setResult(null)
    setUrl('')
    setText('')
    setTitle('')
    setCompany('')
  }

  return (
    <div className={styles.wrap}>
      {!result && (
        <>
          <h1 className={styles.title}>Новое сопроводительное письмо</h1>
          <p className={styles.subtitle}>
            {mode === 'url'
              ? 'Вставьте ссылку на вакансию hh.ru — остальное сделает сервис.'
              : 'Вставьте текст вакансии — остальное сделает сервис.'}
          </p>

          <div className={styles.segmented} role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'url'}
              className={`${styles.segment} ${mode === 'url' ? styles.segmentActive : ''}`}
              onClick={() => handleModeChange('url')}
              disabled={loading}
            >
              Ссылка на hh
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'text'}
              className={`${styles.segment} ${mode === 'text' ? styles.segmentActive : ''}`}
              onClick={() => handleModeChange('text')}
              disabled={loading}
            >
              Текст вакансии
            </button>
          </div>

          <form className={styles.form} onSubmit={handleSubmit}>
            {mode === 'url' ? (
              <input
                type="url"
                className={styles.input}
                placeholder="https://hh.ru/vacancy/123456"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                disabled={loading}
                required
              />
            ) : (
              <>
                <AutoTextarea
                  value={text}
                  onChange={setText}
                  placeholder="Вставьте текст вакансии…"
                  disabled={loading}
                />
                <div className={styles.row}>
                  <input
                    type="text"
                    className={styles.input}
                    placeholder="Название (необязательно)"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    disabled={loading}
                  />
                  <input
                    type="text"
                    className={styles.input}
                    placeholder="Компания (необязательно)"
                    value={company}
                    onChange={(e) => setCompany(e.target.value)}
                    disabled={loading}
                  />
                </div>
              </>
            )}
            <button type="submit" className={styles.button} disabled={loading || !canSubmit}>
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
          <button type="button" className={styles.newBtn} onClick={resetForm}>
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
