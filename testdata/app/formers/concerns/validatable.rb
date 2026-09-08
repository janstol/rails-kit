module Validatable
  def validate_presence_of_all(*attrs)
    attrs.all? { |attr| send(attr).present? }
  end

  def validation_summary
    errors.full_messages.join(", ")
  end
end
