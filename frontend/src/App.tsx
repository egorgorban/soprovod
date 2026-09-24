import { NavLink, Route, Routes } from 'react-router-dom'
import styles from './App.module.css'
import NewApplication from './pages/NewApplication'
import History from './pages/History'
import ApplicationView from './pages/ApplicationView'

function App() {
  return (
    <>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <span className={styles.logo}>Сопровод</span>
          <nav className={styles.nav}>
            <NavLink to="/" end className={({ isActive }) => (isActive ? styles.navLinkActive : styles.navLink)}>
              Новое
            </NavLink>
            <NavLink
              to="/history"
              className={({ isActive }) => (isActive ? styles.navLinkActive : styles.navLink)}
            >
              История
            </NavLink>
          </nav>
        </div>
      </header>
      <main className={styles.main}>
        <Routes>
          <Route path="/" element={<NewApplication />} />
          <Route path="/history" element={<History />} />
          <Route path="/applications/:id" element={<ApplicationView />} />
        </Routes>
      </main>
    </>
  )
}

export default App
