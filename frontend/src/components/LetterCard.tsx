import { useEffect, useRef, useState } from 'react'
import type { Application, Vacancy } from '../types'
import { api, ApiRequestError } from '../api'
import styles from './LetterCard.module.css'

interface Props {
  application: Application
  vacancy: Vacancy
  onSaved?: (application: Application) => void
}

function formatSalary(salary: string | null): string | null {
  return salary && salary.trim() ? salary : null
}

function AutoTextarea({
  value,
  onChange,
}: {
  value: string
  onChange: (v: string) => void
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
      spellCheck
    />
  )
}

export default function LetterCard({ application, vacancy, onSaved }: Props) {
  const initialText = application.edited_text ?? application.generated_text ?? ''
  const [text, setText] = useState(initialText)
  const [copyState, setCopyState] = useState<'idle' | 'copied'>('idle')
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [saveError, setSaveError] = useState<string | null>(null)
  const [showExperience, setShowExperience] = useState(false)
  const [showDescription, setShowDescription] = useState(false)

  useEffect(() => {
    setText(application.edited_text ?? application.generated_text ?? '')
  }, [application.id, application.edited_text, application.generated_text])

  const dirty = text !== (application.edited_text ?? application.generated_text ?? '')

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(text)
      setCopyState('copied')
      setTimeout(() => setCopyState('idle'), 1600)
    } catch {
      // ignore clipboard errors
    }
  }

  async function handleSave() {
    setSaveState('saving')
    setSaveError(null)
    try {
      const { application: updated } = await api.updateApplication(application.id, text)
      setSaveState('saved')
      onSaved?.(updated)
      setTimeout(() => setSaveState('idle'), 1600)
    } catch (err) {
      setSaveState('error')
      setSaveError(err instanceof ApiRequestError ? err.message : 'Не удалось сохранить')
    }
  }

  const salary = formatSalary(vacancy.salary)
  const experience = application.llm_output?.relevant_experience ?? []
  const stack = application.llm_output?.stack ?? []

  return (
    <div className={styles.card}>
      <div className={styles.vacancyHeader}>
        <h2 className={styles.vacancyTitle}>
          <a href={vacancy.url} target="_blank" rel="noreferrer">
            {vacancy.title}
          </a>
        </h2>
        <div className={styles.vacancyMeta}>
          <span>{vacancy.company}</span>
          {salary && (
            <>
              <span className={styles.dot}>·</span>
              <span>{salary}</span>
            </>
          )}
        </div>
      </div>

      {application.status === 'failed' && (
        <div className={styles.errorBox}>
          Не удалось сгенерировать письмо{application.error ? `: ${application.error}` : ''}
        </div>
      )}

      {application.status === 'filtered_out' && (
        <div className={styles.warnBox}>
          Вакансия отфильтрована{application.filter_reason ? `: ${application.filter_reason}` : ''}
        </div>
      )}

      {(application.status === 'generated' || text) && (
        <>
          <AutoTextarea value={text} onChange={setText} />

          <div className={styles.actions}>
            <button type="button" className={styles.primaryBtn} onClick={handleCopy}>
              {copyState === 'copied' ? 'Скопировано' : 'Скопировать'}
            </button>
            <button
              type="button"
              className={styles.secondaryBtn}
              onClick={handleSave}
              disabled={!dirty || saveState === 'saving'}
            >
              {saveState === 'saving' ? 'Сохранение…' : saveState === 'saved' ? 'Сохранено' : 'Сохранить'}
            </button>
            {saveState === 'error' && <span className={styles.saveError}>{saveError}</span>}
          </div>

          {(experience.length > 0 || stack.length > 0) && (
            <div className={styles.collapsible}>
              <button
                type="button"
                className={styles.collapsibleToggle}
                onClick={() => setShowExperience((v) => !v)}
              >
                {showExperience ? '▾' : '▸'} Почему этот опыт
              </button>
              {showExperience && (
                <div className={styles.collapsibleBody}>
                  {stack.length > 0 && (
                    <div className={styles.chips}>
                      {stack.map((s) => (
                        <span key={s} className={styles.chip}>
                          {s}
                        </span>
                      ))}
                    </div>
                  )}
                  <ul className={styles.expList}>
                    {experience.map((item, i) => (
                      <li key={i}>
                        <strong>{item.source}:</strong> {item.text}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </>
      )}

      <div className={styles.collapsible}>
        <button type="button" className={styles.collapsibleToggle} onClick={() => setShowDescription((v) => !v)}>
          {showDescription ? '▾' : '▸'} Описание вакансии
        </button>
        {showDescription && (
          <div className={styles.collapsibleBody}>
            {vacancy.key_skills.length > 0 && (
              <div className={styles.chips}>
                {vacancy.key_skills.map((s) => (
                  <span key={s} className={styles.chip}>
                    {s}
                  </span>
                ))}
              </div>
            )}
            <p className={styles.description}>{vacancy.description}</p>
          </div>
        )}
      </div>
    </div>
  )
}
