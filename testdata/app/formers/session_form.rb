class SessionForm
  include ActiveModel::Model

  attr_accessor :email, :password

  validates :email, presence: true
  validates :password, presence: true

  def authenticate
    User.find_by(email: email)&.authenticate(password)
  end
end
