import type { FormEventHandler, ReactNode } from 'react'

type FormProps = {
  title?: string
  onSubmit: FormEventHandler<HTMLFormElement>
  actions?: ReactNode
  children: ReactNode
}

export function Form({ title, onSubmit, actions, children }: FormProps) {
  return (
    <form className="ui-form" onSubmit={onSubmit}>
      {title ? <h3>{title}</h3> : null}
      <div className="ui-form-body">{children}</div>
      {actions ? <div className="ui-form-actions">{actions}</div> : null}
    </form>
  )
}
